# Cloud Networking Attacks and Mitigation: A Comprehensive Technical Guide

## Table of Contents
1. [Foundational Concepts](#foundational-concepts)
2. [Threat Model Framework](#threat-model-framework)
3. [Layer 2 (Data Link) Attacks](#layer-2-attacks)
4. [Layer 3 (Network) Attacks](#layer-3-attacks)
5. [Layer 4 (Transport) Attacks](#layer-4-attacks)
6. [Application Layer Attacks](#application-layer-attacks)
7. [Cloud-Specific Attack Vectors](#cloud-specific-attack-vectors)
8. [Defense-in-Depth Architecture](#defense-in-depth-architecture)
9. [Implementation Strategies](#implementation-strategies)
10. [Testing and Validation](#testing-and-validation)

---

## Foundational Concepts

### Why Cloud Networking is Different

Traditional network security assumes:
- Clear network perimeter
- Known topology
- Controlled infrastructure layer
- Homogeneous trust domains

Cloud networking breaks these assumptions:

```
Traditional Network:
┌─────────────────────────────────┐
│  Organization Network           │
│  ┌──────────────────────────┐   │
│  │ Servers                  │   │
│  │ Known topology           │   │
│  │ Controlled borders       │   │
│  └──────────────────────────┘   │
│  Firewall at perimeter          │
└─────────────────────────────────┘
     ↓ Internet (untrusted)

Cloud Network:
┌──────────────────────────────────────────────────┐
│  Shared Hypervisor Platform                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐          │
│  │Tenant A │  │Tenant B │  │Tenant C │ (VMs)    │
│  │  vNIC   │  │  vNIC   │  │  vNIC   │          │
│  └────┬────┘  └────┬────┘  └────┬────┘          │
│       │            │            │                │
│       └────────────┼────────────┘                │
│                    │                             │
│    Virtual Switch (vSwitch)                      │
│    - Operates at L2/L3                          │
│    - Shared resource                            │
│    - Complex filtering rules                    │
└──────────────────────────────────────────────────┘
       ↓ Cloud Infrastructure (partially untrusted)
```

### Key Differences in Attack Surface

| Aspect | Traditional | Cloud |
|--------|-------------|-------|
| **Guest Isolation** | OS/firewalls | Hypervisor + vNIC filters + security groups |
| **Shared Resources** | Rare | CPU, memory, storage backend |
| **Metadata Service** | N/A | Critical (credentials, instance info) |
| **Control Plane** | Direct access | API-based, harder to audit |
| **Topology Changes** | Planned | Continuous (auto-scaling) |
| **Cross-tenant Traffic** | Through firewalls | Through vSwitch, hypervisor |

### The OSI Model in Cloud Context

```
Layer 7: Application      | Custom app protocols, API attacks
Layer 6: Presentation     | TLS/SSL, certificate attacks
Layer 5: Session          | Session hijacking, replay
────────────────────────────────────────────────────────
Layer 4: Transport        | TCP/UDP floods, port-based attacks
Layer 3: Network          | IP spoofing, routing attacks, DDoS
Layer 2: Data Link        | ARP spoofing, MAC flooding, VLAN hopping
Layer 1: Physical         | VM escape (hypervisor border)
```

---

## Threat Model Framework

### Building Effective Threat Models

A threat model answers:
1. **What assets do I have?** (Data, compute, credentials, reputation)
2. **What are realistic threats?** (Attack vectors specific to my deployment)
3. **Who are plausible attackers?** (Nation-state, competitor, insider, script kiddie)
4. **What's my risk tolerance?** (Detection time, false positive rate, performance cost)

### STRIDE in Cloud Environments

**S - Spoofing**
- IP spoofing at L3
- MAC spoofing at L2
- ARP spoofing
- DNS spoofing
- Instance identity spoofing

**T - Tampering**
- Packet modification in-flight
- MITM attacks on unencrypted channels
- Hypervisor-level interference (rare but possible)
- Metadata service tampering

**R - Repudiation**
- Denying attacks occurred
- Deleting logs
- Spoofed source IP making attribution hard
- Cloud provider logs as source of truth

**I - Information Disclosure**
- Eavesdropping on unencrypted traffic
- Side-channel attacks (cache timing, memory deduplication)
- Cloud provider visibility into customer traffic
- Metadata service exposure

**D - Denial of Service**
- Network-level DDoS
- Application-level DDoS
- Resource exhaustion (connection tables, CPU)
- Disk/network bandwidth saturation

**E - Elevation of Privilege**
- VM escape
- Kernel privilege escalation
- Container breakout
- Metadata service abuse to get credentials

### Risk Quantification

```
Risk = Likelihood × Impact × Detectability

Example:
Attack: ARP spoofing from sibling VM
  Likelihood: High (same hypervisor, local network)
  Impact: High (all traffic between VMs compromised)
  Detectability: Low (hard to notice, same vSwitch)
  Risk Score: High × High × Low = CRITICAL

Mitigation: Deploy DAI (Dynamic ARP Inspection)
  New Likelihood: Very Low (DAI blocks spoofed ARPs)
  New Risk: Very Low × High × Medium = MEDIUM (manageable)
```

---

## Layer 2 (Data Link) Attacks

### MAC Flooding Attack

**How it works:**
A sender floods the switch with frames containing spoofed source MAC addresses. The switch's MAC address table (CAM) has finite capacity (typically 8K-16K entries). When full, the switch enters "fail-open" mode: broadcasts all frames to all ports.

**In cloud context:**
```
┌──────────────────────────────────────────────┐
│         Physical Host (Hypervisor)           │
├──────────────────────────────────────────────┤
│                   vSwitch                    │
│  ┌────────────────────────────────────────┐ │
│  │ MAC Table (limited entries)            │ │
│  │ [MAC] -> [Port]                        │ │
│  │ [MAC] -> [Port]                        │ │
│  │ ... FULL CAPACITY ...                  │ │
│  │ [New MAC] -> BROADCAST (fail-open)    │ │
│  └────────────────────────────────────────┘ │
│  │                                          │
│  ├─ vNIC (VM-A)                             │
│  ├─ vNIC (VM-B)  ← Attacker VM             │
│  ├─ vNIC (VM-C)                             │
│  └─ Physical NIC                            │
└──────────────────────────────────────────────┘

Attack Flow:
VM-B (Attacker): Send frames with spoofed MAC addresses
                 Continuously, thousands per second

vSwitch: MAC table fills up
         Enters fail-open mode
         All future frames broadcast to all ports

Result: VM-A can see traffic meant for VM-C
```

**Why it's dangerous:**
- **Passive eavesdropping becomes possible**: Attacker can sniff all VLAN traffic
- **ARP poisoning becomes easier**: Can predict and poison ARP entries
- **DHCP starvation**: Can exhaust DHCP pools

**Mitigation Strategies:**

1. **Port Security (Physical Switches)**
   ```
   Maximum MACs per port: 1-5
   Violation action: shutdown/restrict/notify
   
   Result: Excess MACs are dropped
   Cost: Configuration, learning, potential false positives
   ```

2. **vSwitch Port Security (Hypervisor Level)**
   - Most modern hypervisors (KVM, ESXi, Hyper-V) support this
   - Limit MACs per vNIC
   - Drop frames exceeding limit
   
   **KVM Example:**
   ```
   vhost-user port configuration:
   max-macs-per-port = 1
   action = drop (not broadcast)
   ```

3. **MAC Address Filtering**
   - Whitelist MACs for each vNIC
   - Reject unknown MACs
   - Update whitelist on legitimate VM operations

4. **Monitoring and Detection**
   - Monitor MAC table utilization on vSwitch
   - Alert if single port learning > threshold
   - Log MAC address changes
   - Correlation: Combine with other signals

---

### ARP Spoofing / ARP Poisoning

**Fundamental Issue:**
ARP (Address Resolution Protocol) has no authentication. Any host can claim any IP-to-MAC mapping.

```
Normal ARP Flow:
Host A: "Who has 10.0.0.5?"         (ARP Request)
Host B: "I'm 10.0.0.5, I'm aa:bb:cc:dd:ee:ff"  (ARP Reply)
Host A: Updates ARP cache
        IP 10.0.0.5 → MAC aa:bb:cc:dd:ee:ff

Attack ARP Spoofing:
Attacker C: "I'm 10.0.0.5, I'm 11:22:33:44:55:66"  (Unsolicited ARP)
Host A: "Oh, IP mapping changed, update cache"
        IP 10.0.0.5 → MAC 11:22:33:44:55:66  (WRONG!)
Host A: Next packet to 10.0.0.5 goes to attacker
```

**Cloud-Specific Scenario:**
```
┌─────────────────────────────────────────────────┐
│  Host (vSwitch + physical NICs)                 │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌──────────┐      ┌──────────┐                │
│  │ VM-Prod  │      │ VM-Att   │                │
│  │10.0.0.10 │      │10.0.0.20 │                │
│  │aa:aa:... │      │cc:cc:... │                │
│  └─────┬────┘      └─────┬────┘                │
│        │                 │                      │
│        └─────┬───────────┘                      │
│              │                                  │
│              vSwitch (Layer 2 bridge)          │
│              ARP Table: Learning from frames   │
│                                                 │
└─────────────────────────────────────────────────┘

Step 1: Attacker VM sends ARP spoofed frame
        Source MAC: cc:cc:cc:cc:cc:cc
        ARP Payload: IP 10.0.0.10 is at cc:cc:cc:cc:cc:cc
        
Step 2: vSwitch forwards to all ports in VLAN
        
Step 3: Production VM receives spoofed ARP
        Updates ARP cache: 10.0.0.10 → cc:cc:cc:cc:cc:cc
        
Step 4: All packets from Prod VM to 10.0.0.10 go to Attacker
        (Or could be different IP, causing redirection)

Result: MITM attack, credential theft, data exfiltration
```

**Why This Matters in Cloud:**
- Intra-host (same hypervisor) attacks are possible
- Limited VLAN boundary enforcement
- VMs not expecting to defend against Layer 2 attacks
- Often run OS with minimal ARP protection

**Mitigation Strategies:**

1. **Dynamic ARP Inspection (DAI)**
   
   How it works:
   ```
   Premise: Legitimate ARP must come from DHCP server
            and use IP/MAC pairs we know about
   
   Configuration:
   - Maintain binding database: IP → MAC → VLAN → Port
   - Inspect ARP frames before forwarding
   - Drop ARP if sender IP/MAC not in binding DB
   - Allow ARP from DHCP server (trusted)
   
   Deployment in cloud:
   - vSwitch-level DAI checks before broadcasting ARP
   - Hypervisor validates ARP from VMs
   - Significant CPU cost (inspection at line rate)
   ```

2. **Static ARP Entries** (For critical services)
   ```
   static_arp[default_gateway] = known_mac
   static_arp[dns_server] = known_mac
   
   Benefit: Can't be poisoned
   Cost: Manual management, doesn't scale
   Use: Only for critical destinations
   ```

3. **ARP Cache Timeout Tuning**
   ```
   Short timeout = Quick detection of legitimate changes
   Long timeout = Better stability, harder to poison
   
   Linux default: 60 seconds
   Cloud recommendation: 30-60 seconds
   + Monitor ARP cache thrashing as sign of attack
   ```

4. **Gratuitous ARP Rate Limiting**
   ```
   Gratuitous ARP: Unrequested ARP frame, IP tells everyone its MAC
   Legitimate uses: Host startup, failover, IP migration
   Attack use: Continuous spoofing
   
   Mitigation:
   - Limit gratuitous ARP per source to N per second
   - Drop excess
   - Alert on high rates
   ```

5. **Encapsulation + Isolation**
   ```
   Don't rely on L2 security alone
   
   - Encrypt all inter-VM traffic (WireGuard, IPSec)
   - Use overlay networks (VXLAN, Geneve) with strong endpoints
   - Implement mutable endpoint verification
   - Add L3/L4 authentication (TLS, mutual TLS)
   
   Result: Even if ARP is spoofed, packets are encrypted
           Attacker gets ciphertext, not plaintext
   ```

---

### VLAN Hopping

**Background:**
VLANs (Virtual LANs) segment Layer 2 broadcast domains. Each VM should only see frames tagged with its VLAN.

**Attack: Trunk Negotiation**
```
VLAN Setup:
┌─────────────────────────────────────┐
│  Physical Switch                    │
├─────────────────────────────────────┤
│  Port 1: Access VLAN 10             │
│  Port 2: Access VLAN 20             │
│  Port 3: Trunk (all VLANs)          │
│  Port 4: Trunk (all VLANs)          │
└─────────────────────────────────────┘

Attack (CVE-2004-3154 - Switch Spoofing):
Attacker VM sends DTP (Dynamic Trunking Protocol) frame
  "I want to be a trunk port!"
  
If switch allows DTP negotiation on access port:
  Switch: "OK, port becomes trunk"
  Attacker VM: Now receives all VLAN traffic!
  
Result: Lateral movement across VLANs
```

**Attack: Double Tagging**
```
Frame structure:
┌──────────────────────────────────────┐
│ Outer VLAN Tag (Attacker's VLAN)     │  ← Switch removes this
│ ┌────────────────────────────────────┤
│ │ Inner VLAN Tag (Target VLAN)       │  ← Switch doesn't see, forwards
│ │ ┌─────────────────────────────────┐│
│ │ │ Payload (reaches target VLAN!)  ││
│ │ └─────────────────────────────────┘│
│ └────────────────────────────────────┘
└──────────────────────────────────────┘

Example:
Attacker in VLAN 10 wants to send to VLAN 20 (isolated)

Frame:
  Outer tag: VLAN 10 (legitimate)
  Inner tag: VLAN 20 (hidden)
  
Physical switch removes outer tag, forwards to VLAN 10
But receives back on VLAN 20 link
End device sees inner tag, processes in VLAN 20 context

Prerequisite: Attacker can inject frames (already compromised)
Result: Once compromised, bypass VLAN restrictions
```

**In Cloud Context:**
```
Cloud VLAN Separation:
┌──────────────────────────────────┐
│  Management Network (VLAN 100)   │ ← Highly restricted
│  Hypervisor mgmt vNICs           │
└──────────────────────────────────┘
         │
         └─ Physical Switch
         │
┌────────┴──────────────────────────┐
│ Tenant VLANs (VLAN 200-300)       │
│ ┌──────────┐  ┌──────────┐        │
│ │ Tenant A │  │ Tenant B │        │
│ │ VLAN 200 │  │ VLAN 210 │        │
│ └──────────┘  └──────────┘        │
└───────────────────────────────────┘

Attack Scenario:
1. Attacker compromises Tenant B VM (VLAN 210)
2. Sends double-tagged frame to Management (VLAN 100)
3. Bypasses VLAN isolation
4. Accesses hypervisor management → full host compromise
```

**Mitigation Strategies:**

1. **Disable DTP (Dynamic Trunking Protocol)**
   ```
   Configuration:
   switchport mode access (not negotiate)
   switchport nonegotiate
   
   Effect: Port stays access, can't be negotiated to trunk
   Cost: Admin effort, must apply to all access ports
   Status: Industry best practice, should be default
   ```

2. **VLAN Access Control List (VACL)**
   ```
   VACL: Like ACL but enforced at VLAN boundary
   
   Configuration:
   permit VLAN 10 to VLAN 20: deny
   permit VLAN 20 to VLAN 10: deny
   permit VLAN 10 to gateway: allow
   permit VLAN 20 to gateway: allow
   deny any other inter-VLAN: default deny
   
   Result: Prevents any direct VLAN-to-VLAN communication
   Cost: Significant CPU on switch, complex policies
   ```

3. **Port Security + VLAN Pinning**
   ```
   Idea: Bind MAC address to VLAN
   
   Configuration:
   MAC aa:bb:cc:dd:ee:ff can only appear on VLAN 200
   
   If frame with that MAC appears in VLAN 210:
   → Violation, shutdown/restrict port
   
   Result: Can't move VM to different VLAN without admin change
   Cost: Management burden during VM migration
   ```

4. **Network Segmentation at L3 (Layer 3)**
   ```
   Don't rely on VLAN alone for security boundary
   
   Strategy:
   - VLAN 100 (Mgmt): Only allow from bastion host
   - VLAN 200 (Tenant A): Only outbound, no inbound
   - VLAN 300 (Tenant B): Only outbound, no inbound
   - Inter-VLAN: Through stateful firewall
   
   Firewall rules:
   permit VLAN 200 to VLAN 300: deny (explicit, not implicit)
   permit VLAN 200 to Internet: allow outbound only
   
   Result: Compromise of single VLAN doesn't reach others
   Cost: Firewall throughput, latency, state management
   ```

5. **Hypervisor-Level Enforcement**
   ```
   Modern hypervisors don't rely solely on physical switch VLAN tags
   
   Implementation:
   - vSwitch filters frames by source VLAN
   - Frame from VM in VLAN 200 with tag VLAN 300: DROP
   - Prevents VM from spoofing VLAN membership
   
   Configuration (OVS example):
   ovs-vsctl set port vnet0 vlan_mode=trunk
   ovs-vsctl set port vnet0 vlan_trunks=200
   
   Result: VM can only send frames tagged for its VLAN
   Cost: Per-VM configuration, validation on every frame
   ```

---

## Layer 3 (Network) Attacks

### IP Spoofing

**Fundamental Concept:**
IP spoofing is the ability to send frames with a source IP address that doesn't match the sender's actual IP. Unlike L2, L3 has no inherent authentication.

**Why It's Possible:**
```
The sending stack doesn't verify:
  "Is my IP actually 10.0.0.5?"

It just checks:
  "Do I have 10.0.0.5 as an address on an interface?"

If you can assign IP 10.0.0.1 to a vNIC, you can send frames
claiming to be from 10.0.0.1. No hardware/firmware validation.

Example:
VM in VLAN 200 assigns itself IP 10.0.0.50 (not allocated to it)
Sends TCP SYN claiming source 10.0.0.50
Receiver trusts the IP, responds to 10.0.0.50
Attacker intercepts responses (same VLAN), exploits trust
```

**Cloud Specific Issues:**
```
┌──────────────────────────────────────────────────┐
│  Hypervisor Host                                 │
├──────────────────────────────────────────────────┤
│  vSwitch Configuration Options:                  │
│                                                  │
│  Option 1: Trust vNIC MAC learning              │
│  ┌──────────────────────────────────────────┐   │
│  │ If packet from VM says src IP X,        │   │
│  │ is it possible that VM owns IP X?      │   │
│  │ → Check ARP, DHCP, IP mgmt plane       │   │
│  │ → Validate against config               │   │
│  │ → Drop if impossible                    │   │
│  │                                          │   │
│  │ Cost: Deep inspection, config sync      │   │
│  └──────────────────────────────────────────┘   │
│                                                  │
│  Option 2: Trust nothing, encrypt everything    │
│  ┌──────────────────────────────────────────┐   │
│  │ Spoofed IP is irrelevant                │   │
│  │ All traffic encrypted/authenticated      │   │
│  │ Attacker can send from any IP           │   │
│  │ But can't decrypt, can't authenticate    │   │
│  │                                          │   │
│  │ Cost: Crypto overhead, key management   │   │
│  └──────────────────────────────────────────┘   │
└──────────────────────────────────────────────────┘
```

**Attack: SYN Flood (Consequence of Spoofing)**
```
Normal TCP Handshake:
Client: SYN (sequence=X)
Server: SYN-ACK (ack=X+1, sequence=Y)
Client: ACK (ack=Y+1)
Connection established

SYN Flood with Spoofing:
Attacker: Sends 1000s of SYN packets
          Each with different spoofed source IP
          source: 10.0.0.1, 10.0.0.2, 10.0.0.3, ...
          
Server: Each SYN allocates server resources
        SYN-ACK backlog queue fills
        Half-open connections: 5000, 10000, 50000
        
Result: No legitimate connections can be established
        Server tied up holding state for phantom connections
        CPU consumed handling TCP timeouts
```

**Mitigation Strategies:**

1. **Source IP Validation (Ingress Filtering - BCP 38)**
   ```
   Principle: A packet's source IP should be routable
              via the interface it arrived on
   
   Implementation at hypervisor:
   ┌─────────────────────────────────────┐
   │ Incoming packet from vNIC (vnet0)   │
   │ Source IP: 10.0.0.50                │
   ├─────────────────────────────────────┤
   │ Check: Is vnet0 allowed to send     │
   │        from 10.0.0.50?              │
   │                                     │
   │ Lookup in config database:          │
   │ vnet0 allocated: 10.0.0.100-110     │
   │ 10.0.0.50 not in range              │
   │                                     │
   │ Action: DROP                        │
   └─────────────────────────────────────┘
   
   Configuration (Linux tc/ebtables):
   # Block spoofed IPs per vNIC
   tc filter add dev vnet0 parent ffff: protocol ip prio 1 \
     u32 match ip src 10.0.0.50 action drop
   
   Cost: Per-frame validation, config management
   Effectiveness: Prevents external spoofing, allows internal
   ```

2. **Reverse Path Forwarding (RPF)**
   ```
   Stricter version of ingress filtering
   
   Principle: Can this packet be legitimately received on this interface?
              (Would the reverse traffic go out this interface?)
   
   Loose RPF:
   - Route table lookup on source IP
   - Check: Is there a route to source IP?
   - If yes, accept; if no, drop
   
   Strict RPF:
   - Route table lookup on source IP
   - Check: Is the route through this interface?
   - Only accept if source IP is routable via incoming interface
   
   Deployment in Cloud:
   - Hypervisor applies strict RPF on vNIC ingress
   - Prevents packets from appearing on wrong network segments
   
   Linux configuration:
   sysctl net.ipv4.conf.vnet0.rp_filter = 1  # Strict
   sysctl net.ipv4.conf.vnet0.rp_filter = 2  # Loose
   
   Limitation: Works at local network, not across internet
               (Attacker can spoof from any IP address globally)
   ```

3. **Anti-DDoS at Scale**
   ```
   For internet-facing services:
   
   Approach 1: Rate Limiting
   - Limit SYN packets per second from any source
   - Enforce at network edge (cloud provider DDoS service)
   - Cost: False positives during legitimate traffic spikes
   
   Approach 2: SYN Cookies
   - Server encodes state in SYN-ACK sequence number
   - Doesn't allocate memory until ACK received
   - ACK must contain valid cookie to complete handshake
   
   Linux:
   sysctl net.ipv4.tcp_syncookies = 1
   
   Cost: CPU overhead to generate/validate cookies
   Benefit: Prevents half-open connection exhaustion
   
   Approach 3: Anycast Scrubbing
   - Traffic to service routed through DDoS scrubber network
   - Scrubbers filter attacks, pass legitimate traffic
   - Attacker traffic amplified on internet, filtered at scrubber
   
   Cost: Latency, third-party dependency
   Benefit: Massive scale DDoS mitigation (Terabit level)
   ```

---

### BGP Hijacking / Route Poisoning

**Context:**
BGP (Border Gateway Protocol) is the routing protocol that makes the internet work. Cloud providers use BGP to announce IP ranges.

```
Normal BGP Announcement:
AS 65000 (CloudProvider-A): "I own 203.0.113.0/24, route through me"
AS 65001 (Competitor): "I own 203.0.113.0/24, route through me"

(In reality, only one should own this)

If Competitor announces the same range with:
- Better path characteristics
- Shorter AS path
- Higher local preference

Internet routers might:
→ Reroute all traffic for 203.0.113.0/24 to Competitor
→ Competitor intercepts, reads, might forward

Result: MITM at global scale
```

**Cloud-Specific BGP Attack:**
```
Scenario: Attacker compromises cloud provider datacenter switch

Steps:
1. Attacker gains access to router configuration
   (via SSH compromise, supply chain, insider threat)

2. Attacker injects BGP announcement:
   announce 203.0.113.0/24 with better AS path
   
3. Propagates across internet within minutes

4. Traffic for 203.0.113.0/24 redirects to attacker
   (Could be DNS servers, load balancers, APIs)

5. Attacker:
   - Reads all traffic
   - Modifies responses
   - Redirects to real destination (to avoid detection)
   - Perfect MITM

Consequences:
- Affects all users globally for those IPs
- Cloud provider loses reputation
- Customers' data compromised
- Trust erosion in BGP itself
```

**Real-World Examples:**
- 2008: YouTube hijack (Pakistan Telecom announced YouTube's IP range)
- 2014: Indosat hijack (announced Google's IP ranges)
- 2017: Level3 hijack (Facebook, Amazon prefixes)
- 2022: AWS prefix hijack (originated from attacker network)

**Mitigation Strategies:**

1. **BGP Route Filtering**
   ```
   Principle: Only accept BGP announcements from legitimate neighbors
   
   Configuration:
   - Import filters: Define which prefixes each AS can announce
   - Export filters: Control what we announce to neighbors
   - Implement prefix lists (whitelists)
   
   Example:
   neighbor 203.0.113.1 route-map FILTER_IN in
   route-map FILTER_IN permit 10
   match ip address prefix-list CUSTOMER_PREFIXES
   set as-path prepend 65000
   
   neighbor 203.0.113.1 route-map FILTER_OUT out
   route-map FILTER_OUT permit 10
   match ip address prefix-list OUR_PREFIXES
   
   Cost: Manual filter maintenance, operational burden
   Protection: Prevents simple hijacks, stops competitors
   Limitation: Doesn't prevent sophisticated attacks
   ```

2. **RPKI (Resource Public Key Infrastructure)**
   ```
   Idea: Digitally sign BGP announcements
   
   How it works:
   ┌──────────────────────────────────────┐
   │ Regional Internet Registry (RIR)     │
   │ apnic, ripe, arin, etc.              │
   │                                      │
   │ Maintains authoritative records:     │
   │ {IP Range} -> {ASN} -> {Public Key}  │
   │                                      │
   │ 203.0.113.0/24 -> AS 65000 -> PK_X   │
   └──────────────────────────────────────┘
             ↑                    ↓
   Authoritative source    BGP router validates
   
   BGP router validation:
   Receives: "AS 65000 announces 203.0.113.0/24, signed with SK_X"
   
   Check: Does RIR say AS 65000 can announce 203.0.113.0/24?
          Is signature valid with published public key?
   
   Valid:   Accept announcement
   Invalid: Reject or downgrade preference
   
   Cost: Requires RIR participation, PKI infrastructure
   Status: Gradually deployed (RPKI is complex)
   Protection: Prevents unauthorized announcements
   ```

3. **ASPA (AS Path Authorization)**
   ```
   Next-generation RPKI extension
   
   Idea: Sign the entire AS path
   
   "AS 65000 via AS 65001 via AS 65002"
   Each AS cryptographically authorizes the next hop
   
   Benefit: Prevents AS path manipulation
   Status: Draft standard, not yet widely deployed
   ```

4. **BGP Monitoring and Anomaly Detection**
   ```
   Can't prevent all attacks, but can detect quickly
   
   Monitoring:
   - RPKI origin validity: How many invalid announcements?
   - AS path changes: Did known prefixes appear in new AS paths?
   - Route stability: Are routes flapping?
   - Prefix hijacks: Did new ASNs announce our prefixes?
   
   Tools:
   - RIPE NCC BGP Routing Report
   - Google Routing Security Monitor
   - CloudFlare Radar
   
   Detection Example:
   Normal: 203.0.113.0/24 announced by AS 65000
   Anomaly: 203.0.113.0/24 announced by AS 65099 (unknown)
   Action: Alert, investigate, contact peers
   
   Detection Time: Seconds to minutes
   Response Time: Minutes to hours
   Gap: Attack might succeed before detection
   ```

5. **Anycast with Diversified Paths**
   ```
   Instead of single IP route:
   
   Multiple ASNs announce the same IP range
   From different geographic locations
   
   Benefits:
   - If one ASN hijacked, traffic goes elsewhere
   - Multiple paths make comprehensive hijack harder
   - Reduces single point of failure
   
   Example:
   AS 65000 (CloudProvider-America): Announces 203.0.113.0/24
   AS 65001 (CloudProvider-Europe):  Announces 203.0.113.0/24
   AS 65002 (CloudProvider-Asia):    Announces 203.0.113.0/24
   
   Attacker hijacks one:
   → Some traffic rerouted
   → Other traffic still reaches real destination
   → Partial outage, not total MITM
   
   Cost: Infrastructure complexity, coordination
   Protection: Reduces impact of single hijack
   ```

---

### DDoS Attacks (Volumetric, Protocol, Application)

**Volumetric DDoS:**
Attacker floods network link with traffic to exhaust bandwidth.

```
Attack Architecture:
┌─────────────────┐
│ Botnet (1000s)  │
└────────┬────────┘
         │ (Send to target)
         ↓
   ┌──────────────┐
   │ Target       │
   │ ISP Link:    │
   │ 10 Gbit      │
   └──────────────┘

Botnet sends 100 Gbit of traffic
Target ISP link can handle 10 Gbit
Traffic drops: ISP drops excess packets
Legitimate users can't connect
```

**Types:**

1. **UDP Flood**
   ```
   Attacker sends thousands of UDP packets/second
   Target must process each (even if protocol doesn't expect it)
   Application might try to respond
   
   Linux receiving UDP on closed port:
   → ICMP "port unreachable" generated
   → CPU consumed sending ICMP
   → If spoofed source, ICMP goes to victim (source)
   ```

2. **DNS Amplification**
   ```
   Open DNS Resolver attack:
   
   Attacker: Queries public DNS resolver
   Query from: Spoofed source (victim's IP)
   Query for: Large record (ANY record is 50KB+)
   
   DNS resolver: Responds to victim with 50KB answer
   
   Attack:
   1000 queries to open DNS resolvers
   50KB each = 50 MB attack from 1 Mbit of query traffic
   Amplification factor: 50x
   ```

3. **Protocol-Based DDoS (SYN Flood, Ping of Death)**
   ```
   SYN Flood: Already discussed
   
   Ping of Death: Send ICMP echo larger than max IP packet
   → IP fragmentation needed
   → Reassembly buffer overflow
   → Crash (old systems)
   
   Slowloris (Application layer):
   - Open HTTP connection
   - Send headers slowly
   - Server holds connection open
   - Attacker opens 1000s of slow connections
   - Server connection pool exhausted
   - Legitimate requests rejected
   ```

**Application-Layer DDoS:**
```
HTTP Flooding:
GET / HTTP/1.1
Host: target.com
[repeating, thousands per second]

Application sees legitimate HTTP requests
Must process each (parse, route, compute)
CPU on app server exhausted
Web server response time → timeout
Legitimate users see 502 Bad Gateway
```

**Mitigation Strategies:**

1. **ISP-Level DDoS Mitigation**
   ```
   Most effective for volumetric attacks
   
   Implementation:
   - ISP detects abnormal traffic to customer
   - Routes traffic through scrubbing center
   - Filters drop:
     * Traffic with invalid source IPs (spoofed)
     * Excess traffic patterns
     * Known attack signatures
   - Passes legitimate traffic back to customer
   
   Deployment:
   - Transparent (automatic)
   - Or opt-in with BGP reroute
   
   Cost: ISP provides as service (paid)
   Protection: Stops attacks before they hit target ISP link
   
   Providers:
   - Cloudflare
   - Akamai
   - AWS Shield Advanced
   - Imperva
   ```

2. **Rate Limiting**
   ```
   At application level:
   
   By IP address:
   - Max 100 requests/second per IP
   - Burst allowance: 1000 per second (short spike)
   - Above threshold: Drop or 429 (Too Many Requests)
   
   By connection:
   - Max 50 concurrent connections per IP
   - Above threshold: New connections refused
   
   By resource:
   - POST /api/login: Max 5 attempts per minute per IP
   - GET /search: Max 100 searches per minute per user
   
   Implementation (nginx example):
   limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
   location /api/ {
       limit_req zone=api burst=50 nodelay;
   }
   
   Cost: False positives (legitimate users from same IP)
         Doesn't scale to distributed attacks
   Protection: Mitigates single-source attacks
   ```

3. **Anycast + Distributed Scrubbing**
   ```
   Deployment:
   ┌─────────────────┐
   │ Attacker Botnet │
   └────────┬────────┘
            │
    ┌───────┴────────┐
    │ BGP propagates │
    │ Route to ANY   │
    └───────┬────────┘
    ┌───────┴──────────────────────┐
    │                              │
   Scrubber-US          Scrubber-ASIA
   - Filters attack      - Filters attack
   - Forwards clean      - Forwards clean
    │                              │
    └──────────┬────────┬──────────┘
               │        │
           Origin AS   Origin AS
           (real svc)  (real svc)
   
   Benefits:
   - Attacks distributed across scrubber network
   - Single scrubber doesn't see full attack
   - Better filtering per scrubber
   - Geographic diversity
   
   Cost: Complex setup, geo-distribution
   Protection: Handles Terabit-scale attacks
   ```

4. **Connection State Management**
   ```
   TCP Connection Tracking:
   ┌──────────────────────────────────┐
   │ Firewall Connection State Table   │
   │                                  │
   │ SYN_RECEIVED: 500 connections    │
   │ ESTABLISHED:  2000 connections   │
   │ TIME_WAIT:    1000 connections   │
   │ TOTAL:        3500/5000 max      │
   └──────────────────────────────────┘
   
   SYN flood detection:
   - Monitor ratio: SYN_RECEIVED / ESTABLISHED
   - Normal: ~1% (quick handshake completion)
   - Attack: ~50%+ (half-open connections)
   
   Response:
   - Drop new SYN packets from sources with many pending
   - Enable SYN cookies
   - Reduce SYN timeout (faster cleanup)
   
   Linux tuning:
   sysctl net.ipv4.tcp_max_syn_backlog = 8096
   sysctl net.ipv4.tcp_synack_retries = 2   # faster timeout
   sysctl net.ipv4.tcp_syncookies = 1
   ```

5. **Application-Level Detection**
   ```
   Beyond rate limiting, detect attack patterns:
   
   Pattern 1: All requests from single IP
   - Normal: Requests from 1000s of different IPs
   - Attack: 99% from 10 IPs
   - Action: Challenge with CAPTCHA, JS computation
   
   Pattern 2: Request distribution anomaly
   - Normal: Diverse URLs, diverse operations
   - Attack: Repeated URL, same operation
   - Action: Drop matching requests
   
   Pattern 3: Client fingerprinting
   - Normal: Different User-Agent, Accept-Language, TLS ciphers
   - Attack: Identical clients, unusual patterns
   - Action: Rate limit suspicious fingerprints
   
   Implementation:
   - Web Application Firewall (WAF)
   - Machine learning models (detect novel attacks)
   - Signature-based blocking (known attack patterns)
   ```

---

## Layer 4 (Transport) Attacks

### TCP Connection Hijacking

**Fundamental Issue:**
TCP sequence numbers are predictable in some implementations, or can be guessed.

```
Normal TCP Exchange:
┌──────────────┐
│ Client       │ seq=1000
│              ├──────SYN──────────────┐
│              │              ┌────────┤Server
│              │              │ seq=2000
│              │◄──────SYN-ACK─────────┤
│              │ ack=1001
│              │
│              │ seq=1001
│              ├─────ACK───────────────┐
│              │ ack=2001              │
│              │                       │
│ Connection established ◄─────────────┤
│ Both sides agree on sequence numbers
└──────────────┘                       └

Attack - Sequence Prediction:
Attacker predicts sequence numbers
Sends packet claiming to be from Client
seq=1001, ack=2001 (valid)
Legitimate client's next packet: seq=1002
Server receives: seq=1001 (from attacker), seq=1002 (from client)
Server is confused, might accept attacker's data first
```

**Cloud Scenario:**
```
Attacker on same hypervisor as victim:
- Can use ARP spoofing to intercept traffic
- Sees real sequence numbers
- Sends RST or data injection
- Takes over connection

Example: SSH hijacking
1. Client connects to server via SSH
2. Attacker spoofs ACK + DATA packet
3. Sends ssh-rsa key exchange to server
4. Server accepts, completes handshake with attacker
5. Client's subsequent packets ignored (wrong ack)
6. Attacker has control of SSH session
```

**Mitigation Strategies:**

1. **Strong Sequence Number Randomization**
   ```
   Modern Linux (2.6.15+): Uses cryptographically random ISS
   
   ISS (Initial Sequence Number) generation:
   ISS = MD5(source_ip, source_port, dest_ip, dest_port, secret) + time
   
   Result: Each connection has unpredictable sequence numbers
   Cost: Small CPU for MD5 per connection
   Protection: Eliminates sequence prediction attacks
   
   Verification:
   tcpdump on network, capture multiple connections
   TCP sequence numbers should appear random
   (Not incrementing, not predictable pattern)
   ```

2. **TCP Timestamps (RFC 1323)**
   ```
   TCP option: Include timestamp in every packet
   
   Segment:
   ┌──────────────────────────────────────────┐
   │ TCP Header                               │
   │ ├─ Source Port                           │
   │ ├─ Dest Port                             │
   │ ├─ Sequence Number                       │
   │ ├─ Acknowledgment Number                 │
   │ ├─ Flags                                 │
   │ │                                        │
   │ ├─ Options:                              │
   │ │  ├─ MSS: 1460                          │
   │ │  ├─ Timestamp: 12345 (client)          │
   │ │  └─ TSECR: 67890 (echo, server's time) │
   │ └─                                       │
   │ Payload                                  │
   └──────────────────────────────────────────┘
   
   Defense:
   Timestamp must increase per packet (monotonic)
   If attacker sends old timestamp, drop
   Prevents replay of old packets
   
   Linux:
   sysctl net.ipv4.tcp_timestamps = 1
   
   Cost: Extra bytes in every packet (12 bytes)
   Protection: Prevents old packet injection
   ```

3. **Encryption + Authentication (TLS/SSL)**
   ```
   Most important mitigation: Encrypt and authenticate all traffic
   
   Result:
   - Attacker can't inject data (authentication fails)
   - Attacker can't read data (encryption)
   - Attacker can send packets, but they're useless
   
   Deployment:
   - HTTPS for web
   - SSH for admin access
   - TLS for internal APIs
   - mTLS for service-to-service
   
   Cost: CPU overhead, latency
   Protection: Eliminates most connection hijacking attacks
   ```

---

### UDP-Based Attacks

**Stateless Protocol Issue:**
UDP has no connection concept, no handshake, no state.

```
Normal UDP:
Client: UDP packet → Server:5353 (DNS)
Server: Responds

Attack - No connection verification:
Attacker: UDP packet claiming from Client → Server:5353
Server: Processes, responds
Result: Attacker can spoof any source IP
        Server doesn't verify it's real client
```

**Reflection/Amplification Attacks:**
```
Concept:
┌─────────────────┐
│ Attacker Bot    │
├─────────────────┤
│ Target victim:  │
│ 192.0.2.1       │
│                 │
│ Sends to:       │
│ DNS Server      │
│ From: 192.0.2.1 │
│ Query: ANY      │
└────────┬────────┘
         │
         ↓
    ┌──────────────────┐
    │ Open DNS Resolver│
    │                  │
    │ Receives query   │
    │ From 192.0.2.1   │
    │ Responds with    │
    │ 50KB answer ─────┐
    └──────────────────┘
                        │
                        ↓
                    ┌───────────────┐
                    │ Victim        │
                    │ 192.0.2.1     │
                    │ Receives 50KB │
                    │ (×1000s)      │
                    │ Connection    │
                    │ overloaded    │
                    └───────────────┘

Amplification factor: 50KB response / small query = 50x
If attacker sends 1 Mbit query traffic:
→ 50 Mbit attacks victim
If 1000 DNS servers used:
→ 50 Gbit attack from 1 Gbit input
```

**Mitigation Strategies:**

1. **Query Rate Limiting (DNS Servers)**
   ```
   DNS server configuration:
   - Max 10 queries per second per source IP
   - Max ANY queries (large response): 1 per second
   - Drop excess
   
   Result: Limits amplification
   
   Secondary benefit:
   - Legitimate DNS queries not affected (typically <5 qps)
   - Attack traffic blocked
   ```

2. **Source IP Validation**
   ```
   DNS server: Only respond if query looks legitimate
   
   Checks:
   - Query from same network as our nameserver?
   - Query for domain we serve?
   - Query format standard (not unusual options)?
   
   Benefit: Reduces reflection amplification
   Cost: Might block legitimate recursive queries
   ```

3. **Firewall Egress Filtering**
   ```
   Problem: Open DNS servers often are legitimate services
            But can't verify source IP is real
   
   Solution: ISP filters outgoing traffic
   
   Rule: Outgoing DNS response (port 53)
         Must have source IP in ISP's IP range
   
   Result:
   - Attacker inside ISP: Can't spoof source (filtered)
   - Attacker outside: Can't use ISP's DNS servers (different issue)
   
   Real-world issue:
   - Many ISPs don't implement egress filtering
   - DNS servers still vulnerable
   - Open Resolver Project documents these
   ```

4. **Response Rate Limiting (RRL)**
   ```
   DNS server advanced feature:
   
   For each source IP + query type combination:
   - Track response rate
   - If exceeds threshold (16 responses/second typical):
     → Drop responses, respond with minimal answers
   
   Result:
   - Legitimate DNS queries: Still work (queries are sporadic)
   - DDoS amplification: Responses limited
   
   Linux BIND configuration:
   rate-limit {
       responses-per-second 16;
       referrals-per-second 16;
       nxdomains-per-second 5;
       errors-per-second 1;
       all-per-second 30;
       window 15;
   };
   ```

---

## Application Layer Attacks

### DNS-Based Attacks

**DNS Protocol Issue:**
DNS is typically unencrypted (UDP port 53) and unauthenticated.

```
Normal DNS Query:
Client: "What's IP for example.com?" (UDP to 8.8.8.8:53)
       [A record query, unencrypted]

Attacker on path intercepts:
→ Modifies response
→ Sends back attacker's IP for example.com
→ Client connects to attacker instead of real server
```

**Attack: DNS Poisoning**
```
Attacker positions on network path (MITM)

Client: DNS query → Recursive resolver
         "What's IP for bank.example.com?"
         
Attacker intercepts, modifies response:
         "IP is 203.0.113.50" (attacker's server)
         
Client connects to 203.0.113.50 (attacker)
Attacker shows fake login page
Captures credentials

Variant: Cache poisoning
Attacker sends unsolicited DNS response
Resolver caches the malicious record
All subsequent queries return attacker's IP
```

**DNS Amplification (Already covered in L4)**

**Mitigation Strategies:**

1. **DNSSEC (DNS Security Extensions)**
   ```
   Principle: Digitally sign DNS records
   
   How it works:
   ┌────────────────────────────────────┐
   │ Authoritative DNS Server           │
   │                                    │
   │ Record: example.com A 203.0.113.1  │
   │ Signature: RRSIG (RSA-4096)        │
   │                                    │
   │ DNSKEY: Public key for zone        │
   │ DS: Hash of DNSKEY                 │
   └────────────────────────────────────┘
                    │
                    ↓ Published in zone
                    
   Resolver verification:
   1. Receive A record + RRSIG
   2. Get DNSKEY from zone
   3. Validate signature with DNSKEY
   4. Validate DNSKEY with DS from parent zone
   5. If all valid → Accept
   
   Result:
   - Attacker can't modify records (signature fails)
   - Attacker can't inject new records (signature fails)
   - Cache poisoning impossible
   
   Cost: 
   - Deployment complexity (DNSSEC operations)
   - Additional data in responses (RRSIG, DNSKEY)
   - CPU for signature validation
   
   Status:
   - Deployed by ~8% of zones
   - DNSKEY rotation is pain point
   - Some resolvers don't validate (breaks DNSSEC guarantee)
   ```

2. **Resolver Source IP Validation**
   ```
   Idea: Resolver only trusts answers from authoritative servers
   
   Problem: Attacker can spoof any server IP
   Solution: Validate response comes from authoritative IP
   
   Implementation:
   Query example.com A
   → Send to 198.41.0.1 (a.root-servers.net)
   → Response from 198.41.0.1: "Ask 1.2.3.4"
   → Send to 1.2.3.4
   
   If response comes from different IP:
   → Reject (poisoning attempt)
   
   Cost: Requires querying auth servers, not caching
   Protection: Prevents simple poisoning
   Limitation: Doesn't stop sophisticated MITM
   ```

3. **DNS over HTTPS (DoH) / DNS over TLS (DoT)**
   ```
   Principle: Encrypt DNS queries
   
   DoT (RFC 7858):
   Client: TCP 853 to resolver (TLS encrypted)
   Query sent over TLS
   Response sent over TLS
   Eavesdropper can't see what domain you're querying
   
   DoH (RFC 8484):
   Client: HTTPS POST to resolver
   Query embedded in HTTPS request
   Response in HTTPS response
   Harder to distinguish DNS traffic from regular web traffic
   
   Benefits:
   - Encryption: Privacy from ISP/MITM
   - Authentication: TLS cert validates resolver
   - No poisoning: Encrypted response can't be spoofed
   
   Deployment:
   - Client must explicitly use DoH/DoT resolver
   - Most browsers/OSes support (Firefox, Chrome, iOS, Android)
   - Not transparent (unless done by gateway)
   
   Cost: 
   - Latency: HTTPS/TLS overhead
   - Privacy: Resolver sees all queries (or not, if using public resolvers)
   - Centralization: Most users use few public resolvers (Cloudflare, Google)
   
   Status: Rapidly deploying
   ```

4. **DNS Response Size Limiting**
   ```
   Combined with Rate Limiting:
   
   - Limit max response size (EDNS0 UDP buffer)
   - Force TCP for large responses (TCP is harder to amplify)
   - Limit responses per second per source
   
   Result: Amplification attacks less effective
   ```

---

### HTTP/HTTPS Attacks

**Connection Reuse (HTTP/1.1)**
```
HTTP/1.1 Keep-Alive:
Server keeps connection open for multiple requests

GET / HTTP/1.1
...
[Server responds]
[Connection stays open]

GET /api HTTP/1.1
[Reuses same TCP connection]
[Server responds]

Attack - Request Smuggling:
Attacker crafts malformed request that:
- Appears as single request to proxy
- Appears as multiple requests to backend

Example:
Proxy sees:
┌──────────────────────────────┐
│POST /upload HTTP/1.1         │
│Content-Length: 135           │
│Transfer-Encoding: chunked    │
│                              │
│[135 bytes of data]           │
│[Malicious request for 2nd]   │
└──────────────────────────────┘

Proxy: "One request, 135 bytes, good"
→ Forwards to backend

Backend: "Transfer-Encoding: chunked overrides Content-Length"
→ Reads chunks
→ Sees malicious request as separate!

Result:
- Bypasses proxy security checks
- Backend processes unauthorized request
- Proxy unaware of second request
```

**Mitigation:**
1. Normalize headers (remove duplicate encodings)
2. Parse request same way as backend
3. Reject ambiguous requests
4. Test compatibility

---

## Cloud-Specific Attack Vectors

### Metadata Service Attacks

**What is the Metadata Service?**

Every cloud provider has an internal metadata service that VMs can query to get:
- Instance credentials (temporary access keys)
- Instance identity
- Network configuration
- User data scripts

```
AWS EC2 Metadata Service:
┌──────────────────────┐
│ VM Instance          │
├──────────────────────┤
│ Application running  │
│ Needs AWS credentials│
│ to call AWS APIs     │
│                      │
│ Queries:             │
│ curl 169.254.169.254 │
│ /latest/meta-data/   │
│ iam/security-        │
│ credentials/default  │
└──────────┬───────────┘
           │
           ↓ (IP route 169.254.169.254 only on host)
           
┌──────────────────────────────────┐
│ Host OS (Linux Kernel)           │
├──────────────────────────────────┤
│ Route: 169.254.169.254 localhost │
│ Metadata Service Instance        │
│                                  │
│ Returns: JSON with temp keys     │
│ {                                │
│  "AccessKeyId": "AKIA...",       │
│  "SecretAccessKey": "...",       │
│  "Token": "...",                 │
│  "Expiration": "..."             │
│ }                                │
└──────────────────────────────────┘
```

**Attack: Metadata Service Misuse**

```
Scenario: Attacker gains code execution on VM

Step 1: Query metadata service
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/

Step 2: Get credentials
AccessKeyId: AKIAIOSFODNN7EXAMPLE
SecretAccessKey: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
Token: AQoDYXdzEJr...

Step 3: Use credentials
aws --access-key-id AKIA... \
    --secret-access-key wJalr... \
    --session-token AQoD... \
    s3 ls

Step 4: List all S3 buckets, download databases, etc.

Why critical:
- Credentials are temporary (15 min - 1 hour)
- But attacker only needs seconds
- Can use credentials from outside VM
- Can call any AWS API the role allows
- Common to have overly permissive roles
```

**Real-World Example (Capital One Breach, 2019):**
```
Attack chain:
1. SQL injection → RCE on EC2 instance
2. Queried metadata service
3. Got AWS credentials
4. Used credentials to access S3 buckets
5. Downloaded 100M records including SSNs, credit card numbers
6. Impact: 100M+ Americans affected
7. Cost to Capital One: $100M settlement
```

**Mitigation Strategies:**

1. **IMDSv2 (Instance Metadata Service Version 2)**
   ```
   AWS mitigation for this exact attack
   
   IMDSv1 (Vulnerable):
   curl http://169.254.169.254/...
   → No authentication
   
   IMDSv2 (Protected):
   Step 1: Get token
   TOKEN=$(curl -X PUT http://169.254.169.254/latest/api/token \
     -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
   
   Step 2: Use token for queries
   curl -H "X-aws-ec2-metadata-token: $TOKEN" \
     http://169.254.169.254/latest/meta-data/iam/...
   
   Defense: 
   - Metadata queries require token from previous PUT
   - Requires legitimate sequence of calls
   - SSRF (Server-Side Request Forgery) attack can't get token
   - PUT requests blocked by HTTP parsers in many frameworks
   
   Cost: Application changes (few lines)
   Protection: Prevents SSRF → credentials leak
   
   Deployment: AWS recommends v2-only by 2024
   ```

2. **Network-Level Isolation**
   ```
   Idea: Don't rely on application to not query metadata
   
   Implementation:
   - Firewall rules block 169.254.169.254 from containers
   - Network policies deny default pod access
   - Requires explicit allow for metadata queries
   
   Kubernetes example:
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: deny-metadata
   spec:
     podSelector: {}
     policyTypes:
     - Egress
     egress:
     - to:
       - podSelector: {}
     - to:
       - namespaceSelector:
           matchLabels:
             name: kube-system
     - to:
       - podSelector: {}
     - ports:
       - protocol: TCP
         port: 53  # DNS
     - ports:
       - protocol: UDP
         port: 53  # DNS
   
   Result: Pods can't query metadata even if compromised
   Cost: Must explicitly allow for services that need credentials
   ```

3. **Short-Lived Credentials + Audit Logging**
   ```
   Credentials rotation:
   - Refresh every 15 minutes (AWS default)
   - Even if attacker gets credentials, short window
   - Audit log shows which principal used creds
   
   Audit Trail:
   CloudTrail logs all API calls:
   {
     "eventName": "ListBuckets",
     "sourceIPAddress": "192.0.2.1",
     "userIdentity": {
       "principalId": "AIDAI...",
       "sessionContext": {
         "sessionIssueTime": "2024-01-15T10:00:00Z"
       }
     }
   }
   
   Detection:
   - If credentials used from unusual IP → Alert
   - If credentials used to call unusual APIs → Alert
   - If calls from compromised instance → Block immediately
   ```

4. **Role-Based Least Privilege**
   ```
   Instead of:
   {
     "Statement": [{
       "Effect": "Allow",
       "Action": "s3:*",
       "Resource": "*"
     }]
   }
   
   Use:
   {
     "Statement": [{
       "Effect": "Allow",
       "Action": ["s3:GetObject", "s3:PutObject"],
       "Resource": "arn:aws:s3:::my-app-bucket/*"
     }]
   }
   
   Result:
   - Attacker can't access other buckets
   - Attacker can't delete objects
   - Attacker can't modify bucket policies
   
   Cost: Requires understanding of least privilege
   Protection: Limits blast radius of credential compromise
   ```

---

### VM Escape Attacks

**Hypervisor Boundary:**
Virtual machine is supposed to be isolated from:
- Host OS
- Other VMs
- Host hardware

```
Normal Isolation:
┌─────────────────────────────────┐
│ Host OS (Kernel Ring 0)         │
├─────────────────────────────────┤
│ Hypervisor (KVM, ESXi, etc)     │
├──────────┬──────────┬───────────┤
│ VM-A     │ VM-B     │ VM-C      │
│ (Ring 3) │ (Ring 3) │ (Ring 3)  │
│ App      │ App      │ App       │
└──────────┴──────────┴───────────┘

Attack - VM Escape:
If hypervisor has vulnerability:
VM-A → Exploit virt. device → Hypervisor privilege → Read/write VM-B memory
```

**Real Examples:**

```
CVE-2020-8835 (Linux KVM):
Privilege escalation in KVM guest → host kernel access
Impact: Guest VM could read/modify host memory
Fixed: Linux 5.6

CVE-2018-3646 (L1TF - L1 Terminal Fault):
Intel CPU microarchitecture issue
Affects: KVM, Xen, Hyper-V
Attack: Attacker in VM reads SMM memory (System Management Mode)
        Reads host memory, other VMs
Impact: Massive - affected all cloud providers for months
Fixed: CPU microcode update + kvm parameters

CVE-2018-12126 (MSBDS - Microarchitectural Store Buffer Data Sampling):
Side-channel on Intel CPUs
Same cloud, different VM → read another VM's data via cache
Impact: Affects millions of VMs
```

**Why Hypervisor Escapes Matter in Cloud:**

```
Traditional Data Center:
- Single organization owns VMs
- Compromise of one doesn't matter (same org)
- Physical access controls security

Cloud:
- Unknown tenants on same physical host
- Hostile tenants may be present
- VM escape = access to competing companies' data
- Multi-billion dollar impact possible
```

**Mitigation Strategies:**

1. **Hypervisor Updates / Patching**
   ```
   No escape for 0-days, but reduce attack surface:
   
   Practice:
   - Monthly hypervisor patches
   - Microcode updates for CPU issues
   - May require VM migration (downtime)
   
   Cloud provider perspective:
   - Coordinated disclosure: Get patch before public
   - Rapid patching: Deploy within days
   - Tracking: Maintain inventory of vulnerable hosts
   
   For customers:
   - Deploy on latest hypervisor versions
   - Request non-vulnerable hardware
   - Cloud providers: AWS Graviton (ARM), other non-Intel options
   ```

2. **CPU Microcode Updates**
   ```
   For side-channel attacks (Spectre, Meltdown, L1TF, MSBDS):
   CPU microcode loaded at boot provides mitigations
   
   Example: MSBDS mitigation
   - Clear store buffer before context switch
   - Prevent speculative loads from leaking
   - Cost: ~5-10% performance impact on some workloads
   
   Deployment:
   - Distro provides microcode package
   - Loaded by firmware at boot
   - Linux: intel-microcode / amd-microcode package
   ```

3. **Secure Enclave / Trusted Execution Environment (TEE)**
   ```
   Idea: Run security-critical code in isolated CPU enclave
   
   Technologies:
   - Intel SGX (Software Guard Extensions)
   - AMD SEV (Secure Encrypted Virtualization)
   - ARM TrustZone
   
   How it works (Intel SGX):
   ┌─────────────────────────────────┐
   │ VM Memory (untrusted)           │
   ├─────────────────────────────────┤
   │ App can read                    │
   │ Hypervisor can't read           │
   │                                 │
   │ SGX Enclave (trusted):          │
   │ ┌──────────────────────────────┤
   │ │ Crypto keys                  │
   │ │ Sensitive computation        │
   │ │ Encrypted, attested          │
   │ └──────────────────────────────┤
   │ App: "Do crypto in enclave"    │
   │ → Can't see what enclave does  │
   │ → But trust result via attestation
   └─────────────────────────────────┘
   
   Benefits:
   - Confidentiality: Hypervisor can't read enclave memory
   - Integrity: Hypervisor can't modify enclave code
   - Remote attestation: Verify enclave is legitimate
   
   Cost: 
   - Limited memory (SGX: 128-512 MB)
   - Complex to program
   - SGX has had vulnerabilities (side-channels, privileged attacks)
   
   Use case: Key management, compliance-sensitive computation
   ```

4. **Hardware Isolation (Dedicated Hosts)**
   ```
   Instead of relying on hypervisor:
   - Rent entire physical host
   - Your VMs only on that host
   - No hostile tenants
   - Prevents same-host attacks entirely
   
   Trade-off:
   - Cost: 2-3x more expensive
   - Availability: Can't migrate to other hosts (some cases)
   - Management: Customer responsible for utilization
   
   Use case: High-security deployments, compliance requirements
   ```

---

### Kubernetes-Specific Attacks

**What is Kubernetes?**
Container orchestration platform. Runs containers on cluster of machines.

```
Architecture:
┌──────────────────────────────────────┐
│ Kubernetes Control Plane             │
│ - API Server                         │
│ - etcd (database)                    │
│ - Controller Manager                 │
│ - Scheduler                          │
└────────────┬─────────────────────────┘
             │
    ┌────────┴─────────┐
    │                  │
┌───▼──────┐     ┌────▼────┐
│ Worker 1 │     │ Worker 2 │
│ ┌──────┐ │     │ ┌──────┐ │
│ │Pod A │ │     │ │Pod C │ │
│ │Cont  │ │     │ │Cont  │ │
│ └──────┘ │     │ └──────┘ │
│ ┌──────┐ │     │ ┌──────┐ │
│ │Pod B │ │     │ │Pod D │ │
│ │Cont  │ │     │ │Cont  │ │
│ └──────┘ │     │ └──────┘ │
└──────────┘     └──────────┘
```

**Attack: Insecure API Server**

```
Kubernetes API Server: Central control point
- Creates/deletes pods
- Manages secrets
- Configures networking
- Manages RBAC

If exposed on internet without auth:
curl http://master.k8s.local:6443/api/v1/namespaces/default/pods

Response: All pods in cluster
curl http://master.k8s.local:6443/api/v1/secrets

Response: All secrets (database passwords, keys, tokens!)

Attack:
1. Get secrets
2. Extract database credentials
3. Get service account tokens
4. Use tokens to create privileged pods
5. Full cluster compromise
```

**Attack: Privileged Container Escape**

```
Normal container:
┌──────────────────────┐
│ Container            │
├──────────────────────┤
│ UID: 0 (root inside) │
│ But namespaced:      │
│ - Own pid namespace  │
│ - Own network ns     │
│ - Own ipc namespace  │
│ Can't see/control:   │
│ - Host kernel        │
│ - Other containers   │
└──────────────────────┘

Privileged container:
┌──────────────────────┐
│ Container            │
│ --privileged         │
├──────────────────────┤
│ UID: 0 (real root)   │
│ Host kernel access   │
│ Host /dev access     │
│ Can mount / read all │
│ Can load kernel mods │
│ Can access all CPUs  │
└──────────────────────┘

Result: Privileged container = Host compromise
```

**Mitigation Strategies:**

1. **Network Policy**
   ```
   Firewall rules between pods
   
   Example: Deny all by default
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: default-deny
   spec:
     podSelector: {}
     policyTypes:
     - Ingress
   
   Then allow specific:
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: allow-frontend
   spec:
     podSelector:
       matchLabels:
         tier: backend
     ingress:
     - from:
       - podSelector:
           matchLabels:
             tier: frontend
       ports:
       - port: 8080
   
   Result: Frontend can reach backend:8080 only
           All other traffic denied
   ```

2. **RBAC (Role-Based Access Control)**
   ```
   Define what each user/service account can do
   
   Example: App SA can't delete pods
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     name: app-role
   rules:
   - apiGroups: [""]
     resources: ["pods"]
     verbs: ["get", "list", "watch"]
   - apiGroups: [""]
     resources: ["configmaps"]
     verbs: ["get"]
   - apiGroups: [""]
     resources: ["pods", "pods/log"]
     resourceNames: ["my-pod"]
     verbs: ["get"]
   
   Result: App can read pods, but not create/delete/modify
   ```

3. **Pod Security Policy / Pod Security Standards**
   ```
   Restrict what containers can do
   
   Restrict:
   - No --privileged
   - No host namespace access (hostNetwork, hostPID)
   - No host path mounts (except /tmp)
   - Drop insecure capabilities
   - Read-only root filesystem
   
   Example:
   apiVersion: policy/v1beta1
   kind: PodSecurityPolicy
   metadata:
     name: restricted
   spec:
     privileged: false
     allowPrivilegeEscalation: false
     requiredDropCapabilities:
     - ALL
     volumes:
     - 'configMap'
     - 'emptyDir'
     - 'projected'
     - 'secret'
     - 'downwardAPI'
     - 'persistentVolumeClaim'
     hostNetwork: false
     hostIPC: false
     hostPID: false
     runAsUser:
       rule: 'MustRunAsNonRoot'
     seLinux:
       rule: 'MustRunAs'
   ```

4. **Runtime Security (Falco, Sysdig)**
   ```
   Monitor container system calls at runtime
   
   Rules:
   - Alert if container executes shell (unexpected)
   - Alert if container opens network socket (suspicious)
   - Alert if container reads sensitive files (/etc/shadow)
   - Alert if container modifies system binaries
   
   Example Falco rule:
   - rule: Suspicious Process
     desc: Detect suspicious processes
     condition: spawned_process and container and \
               (proc.name in (curl, wget) or proc.args contains "nc")
     output: Suspicious process (user=%user.name proc=%proc.name)
     priority: WARNING
   
   Result: Real-time detection of container compromise
   ```

5. **Image Scanning & Registry Security**
   ```
   Scan container images for vulnerabilities
   
   Tools:
   - Trivy: Scan images for CVEs
   - Snyk: Dependency vulnerabilities
   - Grype: SBOM-based scanning
   
   Process:
   1. Build container image
   2. Scan for vulnerabilities
   3. If critical CVE: Don't push to registry
   4. On pull from registry: Re-scan for new CVEs
   5. If new CVE discovered: Evict running pods
   
   Example:
   $ trivy image python:3.9
   
   Output:
   python:3.9 (debian 11)
   ==================
   Total: 125 Vulnerabilities
   - CRITICAL: 3
   - HIGH: 22
   - ...
   ```

---

## Defense-in-Depth Architecture

### Network Architecture for Cloud Security

**Zero Trust Architecture:**

```
Traditional Trust Model:
┌──────────────────────────────┐
│ Organization Network (Trusted)│
│                              │
│ ┌──────────┐    ┌──────────┐│
│ │Workstation│─── Server    ││
│ └──────────┘    └──────────┘│
│ [Firewall at border]         │
└──────────────────────────────┘
              │
              ↓
    [Internet (Untrusted)]

Problem: Internal communication assumed safe
→ No encryption
→ No authentication
→ Compromised server has access to all others

Zero Trust Model:
┌──────────────────────────────────────┐
│ Organization Network                  │
│                                      │
│ ┌──────────┐  [Encrypted]  ┌──────┐ │
│ │Workstation├──────────────┤Server│ │
│ │ Verified │  [Authn/Authz]│Verified
│ │ mTLS     │  [Audit Log]  │      │ │
│ └──────────┘               └──────┘ │
│                                      │
│ [Firewall blocks all by default]    │
│ [Explicit allow rules only]          │
└──────────────────────────────────────┘

Principle: Never trust, always verify
- All traffic encrypted (TLS)
- All clients authenticated (mTLS, Kerberos)
- All access logged
- Least privilege enforced
```

**Implementation Architecture:**

```
┌─────────────────────────────────────────────────────────────────┐
│ Cloud VPC / Virtual Network                                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ Public Tier (DMZ)                                      │    │
│  │ ┌──────────────────────────────────────────────────┐  │    │
│  │ │ NAT Gateway / Bastion Host                      │  │    │
│  │ │ - Ingress from Internet allowed (limited)       │  │    │
│  │ │ - Egress to App tier only                       │  │    │
│  │ │ - All inbound auth-required                     │  │    │
│  │ └──────────────────────────────────────────────────┘  │    │
│  │ ┌──────────────────────────────────────────────────┐  │    │
│  │ │ Web Tier (LBs, WAF)                            │  │    │
│  │ │ - Public IPs                                    │  │    │
│  │ │ - TLS termination                              │  │    │
│  │ │ - DDoS protection                              │  │    │
│  │ │ - Web Application Firewall                     │  │    │
│  │ │ - Logs to security monitoring                  │  │    │
│  │ └──────────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────────┘    │
│                        ↓ (Firewall rule)                       │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ Application Tier (Private)                            │    │
│  │ ┌──────────────────────────────────────────────────┐  │    │
│  │ │ App Servers                                     │  │    │
│  │ │ - No public IPs                                │  │    │
│  │ │ - mTLS to database tier                       │  │    │
│  │ │ - Service mesh for observability/security     │  │    │
│  │ │ - Secrets from vault (not env vars)           │  │    │
│  │ │ - No outbound to internet                     │  │    │
│  │ └──────────────────────────────────────────────────┘  │    │
│  │ ┌──────────────────────────────────────────────────┐  │    │
│  │ │ Cache Layer (Redis, Memcached)                 │  │    │
│  │ │ - Not exposed to web tier directly             │  │    │
│  │ │ - Accessed through app layer only              │  │    │
│  │ │ - Encrypted with app tier keys                 │  │    │
│  │ └──────────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────────┘    │
│                        ↓ (Firewall rule)                       │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ Database Tier (Private)                              │    │
│  │ ┌──────────────────────────────────────────────────┐  │    │
│  │ │ Database Instances                              │  │    │
│  │ │ - No public IPs                                │  │    │
│  │ │ - Encrypted at rest (KMS key)                 │  │    │
│  │ │ - Encrypted in transit (TLS + mTLS)          │  │    │
│  │ │ - Only app tier can connect (port 3306)      │  │    │
│  │ │ - Replication to standby (encrypted channel)  │  │    │
│  │ │ - Audit logging enabled                       │  │    │
│  │ │ - Automated backups (encrypted)                │  │    │
│  │ │ - No direct admin SSH access                  │  │    │
│  │ └──────────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                  │
│  ┌────────────────────────────────────────────────────────┐    │
│  │ Management Tier (Separate)                            │    │
│  │ ┌──────────────────────────────────────────────────┐  │    │
│  │ │ Bastion / Jumpbox                              │  │    │
│  │ │ - Restricted access (whitelist IPs)            │  │    │
│  │ │ - MFA required                                  │  │    │
│  │ │ │ - VPN required                                │  │    │
│  │ │ - All access logged                             │  │    │
│  │ │ - Isolated from prod traffic                    │  │    │
│  │ │ - Can reach only specific admin ports           │  │    │
│  │ └──────────────────────────────────────────────────┘  │    │
│  └────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

Firewall Rules (Whitelist Model):
Public  → Web: 80, 443
Web     → App: 8080, 8443 (mTLS)
App     → DB: 3306 (mTLS)
DB      → Nowhere
Mgmt    → App,DB: Limited admin ports
App     → Internet: 443 only (for API calls)
DB      → Internet: Denied
Between same tier: Depends on workload
All else: Denied (default deny)
```

---

### Monitoring and Incident Response

**Security Monitoring Stack:**

```
┌──────────────────────────────────────────────────────────┐
│ Data Collection Layer                                    │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ Network Level:
│ - Flow logs (VPC Flow Logs, sFlow, NetFlow)
│ - Packet capture (tcpdump on specific rules)
│ - DNS query logs
│
│ Host Level:
│ - System logs (auditd, journalctl)
│ - File integrity monitoring (osquery, auditbeat)
│ - Process execution logs (auditd, ETW on Windows)
│ - Network connections (netstat, conntrack)
│
│ Application Level:
│ - Web server access logs (nginx, Apache)
│ - Application logs (structured JSON)
│ - API call logs (authenticated user, timestamp, action)
│
│ Cloud Platform Level:
│ - CloudTrail (AWS API calls)
│ - Azure Activity Log
│ - GCP Cloud Audit Logs
│ - Kubernetes audit logs
│
└──────────────────────────────────────────────────────────┘
                     ↓ (ship logs)
┌──────────────────────────────────────────────────────────┐
│ Log Aggregation & Analysis                              │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ Tools: ELK Stack, Splunk, Datadog, New Relic
│ - Index logs (searchable)
│ - Parse structured logs
│ - Extract fields
│ - Store with retention (months to years)
│
└──────────────────────────────────────────────────────────┘
                     ↓
┌──────────────────────────────────────────────────────────┐
│ Detection & Alerting                                    │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ Rules (written in query language):
│
│ Rule 1: Privilege escalation
│ auditlog where uid_before=1000 && uid_after=0
│ → Alert: Privilege escalation detected
│
│ Rule 2: Data exfiltration
│ source=database_tier && destination=internet
│ && protocol!=443
│ → Alert: Unencrypted data flowing to internet
│
│ Rule 3: Suspicious API usage
│ cloudtrail where principal=service_account &&
│ action=DescribeInstances && (num_calls > 100 ||
│ time_since_last_call < 1s)
│ → Alert: Unusual API access pattern
│
│ Rule 4: Failed login brute force
│ authentication_log where result=failure &&
│ same_user && 10+ failures in 60s
│ → Alert: Brute force attempt
│
└──────────────────────────────────────────────────────────┘
                     ↓
┌──────────────────────────────────────────────────────────┐
│ Response & Remediation                                  │
├──────────────────────────────────────────────────────────┤
│                                                          │
│ Automated:
│ - Kill process
│ - Revoke credentials
│ - Isolate VM (network disconnect)
│ - Block IP
│ - Scale down auto-scaling group
│
│ Manual:
│ - On-call engineer alerted
│ - Playbook followed
│ - Evidence collected
│ - Post-mortem conducted
│
└──────────────────────────────────────────────────────────┘
```

**Specific Detection Rules for Cloud Attacks:**

```
1. Metadata Service Abuse
Rule: pods where egress_destination="169.254.169.254"
      && namespace != "kube-system"
Action: Block + Alert

2. ARP Spoofing (Layer 2)
Rule: network_data where arp_source_mac != binding[arp_source_ip]
Action: Drop + Alert (DAI)

3. DNS Poisoning
Rule: dns_response where !dnssec_valid &&
      (rrtype=ANY || large_response)
Action: Log + Alert to DNS team

4. BGP Hijacking
Rule: bgp_announcement where prefix in_whitelist==false ||
      asn_path_length > 3
Action: Alert + Manual review (too risky to auto-respond)

5. VM Escape Detection
Rule: syscall where container_id && (
      syscall in (ptrace, process_vm_readv,
                 mmap_kernel_address) ||
      tries_to_load_kernel_module ||
      accesses_proc_mem)
Action: Kill container + Kill host (too risky to continue)

6. Kubernetes API Abuse
Rule: kubernetes_audit where
      verb in (create, delete, patch) &&
      resource in (ClusterRoleBinding, ClusterRole) &&
      user not in admin_group
Action: Deny + Alert

7. Lateral Movement (East-West Traffic)
Rule: network_flow where
      src_tier != dst_tier &&
      protocol != 443 &&  # Allow HTTPS
      !approved_flows[src_tier][dst_tier]
Action: Alert (too many false positives, watch for patterns)
```

---

## Implementation Strategies

### Deployment Patterns

**Pattern 1: Encrypt Everything in Transit**

```
Implementation:
- TLS for all external communication
- mTLS (mutual TLS) for internal service-to-service
- IPSec for VPN/tunnel traffic

Configuration example (service mesh - Istio):
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
spec:
  mtls:
    mode: STRICT  # Require mTLS

Result:
- Network sniffer can't read data
- MITM can't modify data (encryption + auth)
- Doesn't prevent access, but prevents exploitation
```

**Pattern 2: Audit Everything**

```
Logging requirements:
- All authentication attempts (success and failure)
- All authorization decisions (access allowed/denied)
- All data access (who read what, when)
- All configuration changes
- All privileged operations

Example for database:
Enable audit plugin (MySQL, PostgreSQL, etc.)
Log: username, timestamp, command, affected rows

Example for cloud:
CloudTrail logs all API calls:
- Who (principal ARN/ID)
- What (API call, parameters)
- When (timestamp)
- Where (source IP)
- Outcome (success/error)

Storage:
- Immutable log store (S3, GCS, append-only)
- Encrypted
- Long retention (1-7 years for compliance)
- Regular integrity checks

Analysis:
- Script checks logs for suspicious patterns
- Human review for compliance audits
- Correlation with alerts for incident response
```

**Pattern 3: Secrets Management**

```
Wrong approach:
1. Hardcode in application
2. Store in environment variables
3. Commit to git (even private repo)
4. Distribute to developers

Result: Secrets proliferate, can't be rotated, exposed in repos

Right approach:
┌──────────────────────────────────┐
│ Secrets Manager (Vault, AWS SM)  │
├──────────────────────────────────┤
│ Database passwords               │
│ API keys                         │
│ Certificates                     │
│ Encryption keys                  │
└────────┬─────────────────────────┘
         │
         ├─ Encryption at rest (KMS)
         ├─ Access control (who can read)
         ├─ Audit logging (who accessed)
         ├─ Automatic rotation
         └─ Versioning

Application access:
1. App authenticates (mTLS, IAM role)
2. Requests secret "db/password/prod"
3. Secrets manager verifies permission
4. Returns encrypted secret
5. App decrypts and uses
6. Secrets manager logs access

Benefits:
- Secrets never in code/config
- Can be rotated without app changes
- Access auditable
- Easy to revoke
```

---

### Testing Cloud Network Security

**Approach 1: Vulnerability Testing (Passive)**

```
Test: Identify weaknesses without exploitation

Tools:
- Port scanners (nmap, zmap)
- Vulnerability scanners (Nessus, Qualys, Rapid7)
- Configuration auditors (ScoutSuite, CloudMapper)
- Secret scanners (truffleHog, GitGuardian)

Example scan:
nmap -sS -p- -sV 203.0.113.0/24
→ Identifies open ports, services, versions

Then: Cross-reference against CVE database
→ Which versions have known vulnerabilities

Result: Report of findings, not exploited
→ Safe to run in production (read-only)
→ Can identify critical issues
```

**Approach 2: Penetration Testing (Active)**

```
Controlled, scoped testing with prior approval

Phases:
1. Reconnaissance
   - Map network topology
   - Identify entry points
   - Gather intelligence

2. Scanning
   - Identify open ports
   - Enumerate services
   - Check for misconfigurations

3. Enumeration
   - Connect to services
   - Extract banners
   - Identify users/accounts
   - Map shares, databases

4. Exploitation (AUTHORIZED)
   - Exploit known vulnerabilities
   - Social engineering
   - Physical intrusion (if in scope)

5. Post-Exploitation
   - Establish persistence
   - Escalate privileges
   - Extract sensitive data
   - Lateral movement

6. Cleanup
   - Remove backdoors
   - Restore systems
   - Document findings

Rules of engagement (CRITICAL):
- Signed written approval from management
- Specific scope (don't test unrelated systems)
- Specific timeframe (don't test during business hours)
- No destructive testing (unless explicitly approved)
- No unauthorized access to data
- Report findings confidentially
```

**Approach 3: Red Team Exercises**

```
Simulated adversary attacking system

Different from penetration testing:
- PT: One-time assessment
- RT: Continuous testing, adaptive

Scenario:
- Red team acts as sophisticated attacker
- Blue team defends and detects
- Multiple attack methods
- Tests both detection and response

Example exercise:
Initial access:
- Phishing email with malware
- Blue team: Email detected? Blocked?

Persistence:
- Red team establishes backdoor
- Blue team: Can they detect C2 traffic?

Lateral movement:
- Red team pivots to database server
- Blue team: Can they detect unusual network traffic?

Exfiltration:
- Red team exfils data
- Blue team: Can they detect data leaving network?

Duration:
- Typical: 1-2 weeks
- Continuous: Ongoing program

Debrief:
- What worked? What didn't?
- Gaps in detection?
- Gaps in response?
- Training needed?
```

**Approach 4: Chaos Engineering (Security)**

```
Deliberately break security controls to test resilience

Example tests:
1. Kill TLS certificates (expire them early)
   → Does system still function?
   → Does monitoring alert?
   → Can key rotation happen without outage?

2. Poison DNS resolution
   → Does app fail gracefully?
   → Can app detect MITM?
   → Fallback working?

3. Simulate compromised node
   → Can other nodes detect isolation?
   → Can data be contained?
   → Recovery automatic?

4. Metadata service unavailable
   → Can app function without credentials?
   → Graceful degradation?

5. One-way network partition (BGP hijack simulation)
   → Can quorum be formed?
   → Data consistency maintained?
   → Alerts firing?

Implementation (Gremlin, Chaos Monkey):
chaos_test("kill_tls_cert") {
  cert_name = "api.example.com"
  duration = 300  # seconds
  rollback_on_error = true
  
  on_start {
    expire_cert(cert_name, -1)  # Make already expired
  }
  
  monitor {
    assert_no_outages()
    assert_alert_fired("cert_expiry")
    assert_clients_retrying()
  }
  
  on_end {
    restore_cert(cert_name)
    wait_for_recovery(600)  # 10 minutes
  }
}
```

---

## Conclusion: Building Mental Models

### Key Principles

1. **Assume Breach**
   - Design as if attacker is already inside
   - Defense in depth, not perimeter security
   - Encryption, authentication, audit at every layer

2. **Least Privilege**
   - Every account, service, application should have minimal access
   - Deny by default, allow by exception
   - Regular access review

3. **Encrypt Sensitive Data**
   - At rest: with key stored separately
   - In transit: TLS/mTLS
   - In use: If possible (TEE, FHE, confidential computing)

4. **Detect and Respond**
   - Can't prevent all attacks
   - Focus on early detection
   - Fast response to minimize damage

5. **Security by Design**
   - Not an afterthought
   - Design threat model before implementation
   - Test attacks, don't just prevent them

### Ecosystem Assessment Framework

When evaluating any cloud networking component, ask:

1. **Threat Model**
   - What's being protected?
   - Who's the attacker?
   - What's the impact of compromise?

2. **Attack Surface**
   - What can an attacker interact with?
   - What data does it touch?
   - What privileges does it hold?

3. **Defense Mechanisms**
   - Confidentiality: Encryption, access control
   - Integrity: Authentication, digital signatures
   - Availability: Redundancy, DDoS mitigation
   - Accountability: Audit logging, monitoring

4. **Trade-offs**
   - Security vs. performance
   - Security vs. usability
   - Security vs. cost
   - Which is acceptable for this use case?

5. **Failure Modes**
   - What happens if defenses fail?
   - Graceful degradation?
   - Detection and recovery?

### Continuous Learning Resources

- **RFCs**: Read network protocol specs (RFC 791, 793, 2104, etc.)
- **CVE Database**: Understand real attacks, what failed
- **Mailing lists**: linux-kernel, oss-security discussions
- **Papers**: Academic research on attacks and defenses
- **Source code**: Read hypervisor, kernel, service mesh code
- **Incident reports**: Learn from others' breaches
- **Tool usage**: Deep dive on iptables, tc, tcpdump, wireguard

---

## Appendix: Command Reference

### Network Diagnostics

```bash
# View network interfaces and IP config
ip addr show
ip route show

# Check firewall rules
sudo iptables -L -n -v
sudo firewall-cmd --list-all

# Monitor network traffic
sudo tcpdump -i eth0 -n

# Check network connections
sudo netstat -an | grep ESTABLISHED
sudo ss -an

# Test routing (path to destination)
traceroute 8.8.8.8
mtr -r -c 100 8.8.8.8

# DNS resolution
nslookup example.com
dig example.com

# Network performance
iperf3 -s  # Server
iperf3 -c 192.0.2.1  # Client
```

### Security Auditing

```bash
# Check listening ports/services
sudo netstat -tlnp
sudo lsof -i

# View firewall rules (UFW)
sudo ufw status verbose
sudo ufw show added

# Check SELinux status
sestatus
getsebool -a

# View audit logs
sudo ausearch -m EXECVE
sudo ausearch -m NETWORK

# Check file permissions
ls -la /etc/sudoers
getfacl /var/www

# SSL/TLS certificate info
openssl s_client -connect example.com:443
openssl x509 -in cert.pem -text -noout
```

### Linux Kernel Tuning (Security)

```bash
# Disable IP forwarding (single-homed host)
sysctl net.ipv4.ip_forward=0

# Enable reverse path filtering
sysctl net.ipv4.conf.all.rp_filter=1
sysctl net.ipv4.conf.default.rp_filter=1

# Enable TCP SYN cookies (SYN flood protection)
sysctl net.ipv4.tcp_syncookies=1

# Enable timestamps
sysctl net.ipv4.tcp_timestamps=1

# Reduce TCP timeout for half-open connections
sysctl net.ipv4.tcp_synack_retries=2
sysctl net.ipv4.tcp_retries2=5

# Enable TCP SACK (but be aware of SACK Panic vuln)
sysctl net.ipv4.tcp_sack=1  # Depends on kernel version
```

