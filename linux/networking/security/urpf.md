Unicast Reverse Path Forwarding (uRPF) is a network security feature. When a router receives a packet, uRPF checks the source IP address in the routing table. If the packet arrives on the exact interface the router would use to send traffic back to that source, it passes. If not, the router drops the packet. [1, 2, 3]  
This mechanism is primarily used to prevent IP source address spoofing—a common tactic in Denial of Service (DoS) attacks. [4, 5]  
How It Works 

1. Source Lookup: The router checks the source IP address of incoming packets against the routing table (FIB). 
2. Verification & Action: If the source is unreachable or the route doesn't match the expected interface, the packet is discarded to prevent malicious or malformed traffic from entering the network. [3, 4, 6]  

Key Modes of Operation 
uRPF is typically configured on an interface and acts in different modes depending on network complexity: 

• Strict Mode: The router checks for a valid routing entry and verifies that the packet arrived on the interface the router would use to reach that source. 
• Loose Mode: The router only verifies that a valid route exists for the source IP address in the routing table; it does not check if it arrived on the exact interface. This is ideal for networks with asymmetric routing, where traffic may take different paths in and out. 
• VRF Mode: Limits the checks strictly to a Virtual Routing and Forwarding (VRF) instance. [2]  

Read more about its configuration and implementation on NetworkLessons.com or Cisco Documentation. [1, 6]  
(Note: Depending on the context, "urpf" can also occasionally stand for Unrecognised Provident Fund, a type of retirement fund in Indian tax law.) [10]  


