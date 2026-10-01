# Raspberry Pi 4B as WiFi Extender on Ubuntu Server

Here's the complete setup. Your Pi will have two wireless interfaces:

| Interface | Role | Purpose |
|-----------|------|---------|
| `wlan0` | Client (STA) | Connects to your **scopus** home WiFi |
| `wlan1` | Access Point (AP) | Broadcasts the **extended network** using TP-Link dongle |

---

## Step 1 — Identify your interfaces

```bash
ip link show
iw dev
lsusb
```

Confirm `wlan0` (built-in Pi WiFi) and `wlan1` (TP-Link dongle) are both visible.

---

## Step 2 — Check your TP-Link dongle version (critical!)

```bash
lsusb | grep -i "0cf3\|2357"
```

| USB ID | Chipset | AP mode |
|--------|---------|---------|
| `0cf3:9271` | Atheros AR9271 (v1) | Works out of the box |
| `2357:010c` | Realtek RTL8188EUS (v2/v3) | Needs driver fix |

**If v2/v3 (Realtek), install AP-capable driver:**

```bash
sudo apt install -y dkms git bc build-essential
git clone https://github.com/aircrack-ng/rtl8188eus
cd rtl8188eus
sudo make dkms_install
sudo reboot
```

**Verify AP mode is supported** after reboot:

```bash
sudo iw list | grep -A 10 "Supported interface modes"
# Must show "AP" in the list
```

---

## Step 3 — Install required packages

```bash
sudo apt update && sudo apt install -y hostapd dnsmasq iptables-persistent

# Unmask hostapd (Ubuntu blocks it by default)
sudo systemctl unmask hostapd
sudo systemctl stop hostapd dnsmasq
```

---

## Step 4 — Connect wlan0 to scopus (your home WiFi)

Find your netplan config file:

```bash
ls /etc/netplan/
```

Edit it (usually `00-installer-config.yaml` or `50-cloud-init.yaml`):

```bash
sudo nano /etc/netplan/00-installer-config.yaml
```

```yaml
network:
  version: 2
  renderer: networkd
  wifis:
    wlan0:
      dhcp4: yes
      access-points:
        "scopus":
          password: "your_scopus_password"
```

Apply:

```bash
sudo netplan apply
sleep 5
ping -I wlan0 -c 3 8.8.8.8   # verify internet works through wlan0
```

---

## Step 5 — Assign static IP to wlan1 (the AP interface)

Create a systemd-networkd config:

```bash
sudo nano /etc/systemd/network/10-wlan1.network
```

```ini
[Match]
Name=wlan1

[Network]
Address=192.168.50.1/24
```

Restart networkd:

```bash
sudo systemctl restart systemd-networkd
```

---

## Step 6 — Configure hostapd (Access Point settings)

```bash
sudo nano /etc/hostapd/hostapd.conf
```

```ini
interface=wlan1
driver=nl80211
ssid=scopus_extended
hw_mode=g
channel=6
wmm_enabled=0
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
wpa_passphrase=YourExtenderPassword123
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP
```

> Change `ssid` and `wpa_passphrase` to whatever you want for the extended network.

Point hostapd to that config:

```bash
sudo nano /etc/default/hostapd
# Set this line:
DAEMON_CONF="/etc/hostapd/hostapd.conf"
```

---

## Step 7 — Configure dnsmasq (DHCP for connected devices)

```bash
sudo mv /etc/dnsmasq.conf /etc/dnsmasq.conf.bak
sudo nano /etc/dnsmasq.conf
```

```ini
interface=wlan1
bind-interfaces
dhcp-range=192.168.50.10,192.168.50.100,255.255.255.0,24h
domain=local
address=/gw.local/192.168.50.1
```

---

## Step 8 — Enable IP forwarding

```bash
sudo nano /etc/sysctl.conf
```

Uncomment or add:

```
net.ipv4.ip_forward=1
```

Apply immediately:

```bash
sudo sysctl -p
```

---

## Step 9 — Set up NAT with iptables

```bash
# Allow traffic to flow from extended network → internet
sudo iptables -t nat -A POSTROUTING -o wlan0 -j MASQUERADE
sudo iptables -A FORWARD -i wlan0 -o wlan1 -m state --state RELATED,ESTABLISHED -j ACCEPT
sudo iptables -A FORWARD -i wlan1 -o wlan0 -j ACCEPT

# Save rules so they survive reboot
sudo sh -c "iptables-save > /etc/iptables/rules.v4"
```

---

## Step 10 — Start and enable all services

```bash
sudo systemctl enable hostapd dnsmasq
sudo systemctl start hostapd dnsmasq

# Check status
sudo systemctl status hostapd
sudo systemctl status dnsmasq
```

---

## Verify everything is working

```bash
# Is AP broadcasting?
sudo iw dev wlan1 info

# Is wlan0 connected to scopus?
ip addr show wlan0

# Is IP forwarding on?
cat /proc/sys/net/ipv4/ip_forward    # should print: 1

# Is DHCP running?
sudo systemctl status dnsmasq
```

Now connect your phone/laptop to **`scopus_extended`** — it should get an IP in the `192.168.50.x` range and have full internet.

---

## Troubleshooting

| Problem | Command to diagnose |
|--------|-------------------|
| hostapd fails to start | `journalctl -xe \| grep hostapd` |
| No internet on extended WiFi | `sudo iptables -t nat -L -v` |
| wlan1 missing | `dmesg \| tail -30` — replug dongle |
| Dongle not in AP mode | `iw list \| grep -A10 "interface modes"` |
| wlan0 not connecting to scopus | `sudo journalctl -u systemd-networkd -f` |

---

**One important note:** The TP-Link TL-WN722N v2/v3 is known to be tricky with AP mode — if hostapd still fails after the driver install, check that the `rtl8188eus` module is loaded with `lsmod | grep 8188` and that it was compiled with AP mode support.

Yes, exactly! That's the correct approach. Here's a clean summary of the data flow:

```
Internet
   ↓
[Home Router - scopus]
   ↓ (WiFi)
[wlan0 - Pi onboard chip]  ←→  NAT/IP Forwarding  ←→  [wlan1 - TP-Link dongle AP]
                                                              ↑ (WiFi)
                                                     Phones / Laptops connect here
```

---

## Clean Step-by-Step (Your Exact Setup)

### 1. Check interfaces

```bash
ip link show
```
You should see:
- `wlan0` → Pi's onboard chip (connects to **scopus**)
- `wlan1` → TP-Link dongle (acts as **Access Point**)

---

### 2. Check TP-Link dongle version

```bash
lsusb | grep -i "tp-link\|0cf3\|2357"
```

- **v1** → Chipset `0cf3:9271` (Atheros) → works immediately ✅
- **v2/v3** → Chipset `2357:010c` (Realtek) → needs extra driver ⚠️

**If v2/v3, install the driver first:**

```bash
sudo apt install -y dkms git bc build-essential
git clone https://github.com/aircrack-ng/rtl8188eus
cd rtl8188eus
sudo make dkms_install
sudo reboot
```

Then verify AP mode is supported:

```bash
sudo iw list | grep -A 10 "Supported interface modes"
# Must show "AP"
```

---

### 3. Install packages

```bash
sudo apt update && sudo apt install -y hostapd dnsmasq iptables-persistent
sudo systemctl unmask hostapd
sudo systemctl stop hostapd dnsmasq
```

---

### 4. Connect wlan0 → scopus (home WiFi)

```bash
sudo nano /etc/netplan/50-cloud-init.yaml
```

```yaml
network:
  version: 2
  renderer: networkd
  wifis:
    wlan0:
      dhcp4: yes
      access-points:
        "scopus":
          password: "your_home_wifi_password"
```

```bash
sudo netplan apply
ping -I wlan0 -c 3 8.8.8.8     # must work before continuing
```

---

### 5. Give wlan1 a static IP

```bash
sudo nano /etc/systemd/network/10-wlan1.network
```

```ini
[Match]
Name=wlan1

[Network]
Address=192.168.50.1/24
```

```bash
sudo systemctl restart systemd-networkd
```

---

### 6. Configure hostapd (TP-Link as AP)

```bash
sudo nano /etc/hostapd/hostapd.conf
```

```ini
interface=wlan1
driver=nl80211
ssid=MyExtendedWiFi
hw_mode=g
channel=6
wpa=2
wpa_passphrase=MyExtenderPassword
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
```

```bash
# Tell hostapd where the config is
echo 'DAEMON_CONF="/etc/hostapd/hostapd.conf"' | sudo tee -a /etc/default/hostapd
```

---

### 7. Configure dnsmasq (gives IPs to devices connecting to TP-Link)

```bash
sudo mv /etc/dnsmasq.conf /etc/dnsmasq.conf.bak
sudo nano /etc/dnsmasq.conf
```

```ini
interface=wlan1
bind-interfaces
dhcp-range=192.168.50.10,192.168.50.100,255.255.255.0,24h
```

---

### 8. Enable IP forwarding

```bash
sudo sed -i 's/#net.ipv4.ip_forward=1/net.ipv4.ip_forward=1/' /etc/sysctl.conf
sudo sysctl -p
```

---

### 9. NAT rules (route traffic wlan1 → wlan0)

```bash
sudo iptables -t nat -A POSTROUTING -o wlan0 -j MASQUERADE
sudo iptables -A FORWARD -i wlan1 -o wlan0 -j ACCEPT
sudo iptables -A FORWARD -i wlan0 -o wlan1 -m state --state RELATED,ESTABLISHED -j ACCEPT

# Save so rules survive reboot
sudo sh -c "iptables-save > /etc/iptables/rules.v4"
```

---

### 10. Start everything

```bash
sudo systemctl enable hostapd dnsmasq
sudo systemctl start hostapd dnsmasq

# Check both are active
sudo systemctl status hostapd
sudo systemctl status dnsmasq
```

---

## Test it

On your phone/laptop, connect to **`MyExtendedWiFi`** with your chosen password → you should get an IP like `192.168.50.x` and have full internet access.

```bash
# On the Pi, monitor connected clients
sudo iw dev wlan1 station dump
```

That's your full WiFi extender — **scopus → Pi → TP-Link → your devices**. 🎉

# Complete Production Setup — TP-Link v2 on Ubuntu Server Pi 4B

## What to expect

| Item | Reality |
|------|---------|
| TL-WN722N v2 max speed | ~40–60 Mbps real-world (N150) |
| After power outage | Auto-recovers in ~60–90 sec |
| Biggest speed trick | Connect **wlan0 to 5GHz** on your home router |

---

## Phase 1 — Fix the v2 Driver (AP Mode)

The stock kernel driver (`r8188eu`) **blocks AP mode**. You must replace it.

```bash
# Full system update first
sudo apt update && sudo apt full-upgrade -y

# Build tools
sudo apt install -y dkms git bc build-essential linux-headers-$(uname -r)

# Remove conflicting packages
sudo apt remove -y r8188eu-dkms 2>/dev/null || true

# Install patched driver via DKMS (auto-rebuilds on kernel updates)
git clone https://github.com/aircrack-ng/rtl8188eus /tmp/rtl8188eus
cd /tmp/rtl8188eus
sudo make dkms_install

# Blacklist the broken stock driver
echo "blacklist r8188eu" | sudo tee /etc/modprobe.d/blacklist-r8188eu.conf
echo "blacklist rtl8188eu" | sudo tee -a /etc/modprobe.d/blacklist-r8188eu.conf

# Update initramfs so blacklist applies at boot
sudo update-initramfs -u

sudo reboot
```

**After reboot — verify driver is correct:**

```bash
# Should show "8188eu", NOT "r8188eu"
lsmod | grep 8188

# Must show "AP" in the list
sudo iw list | grep -A 15 "Supported interface modes"

# Both interfaces must exist
ip link show | grep wlan
```

---

## Phase 2 — Prevent USB Dongle from Disconnecting

The dongle can silently disconnect due to USB power saving. Fix this permanently:

```bash
sudo nano /etc/udev/rules.d/99-usb-power.rules
```

```
ACTION=="add", SUBSYSTEM=="usb", ATTR{idVendor}=="2357", ATTR{idProduct}=="010c", ATTR{power/autosuspend}="-1"
```

```bash
sudo udevadm control --reload-rules && sudo udevadm trigger
```

---

## Phase 3 — Connect wlan0 to Home WiFi (Use 5GHz!)

> **Speed tip:** Pi 4B supports 5GHz. Connecting wlan0 to your router's 5GHz band avoids interference with wlan1 (2.4GHz dongle) and roughly **doubles your throughput**.

```bash
ls /etc/netplan/     # find your config file name
sudo nano /etc/netplan/50-cloud-init.yaml
```

```yaml
network:
  version: 2
  renderer: networkd
  wifis:
    wlan0:
      dhcp4: yes
      dhcp4-overrides:
        route-metric: 100
      access-points:
        "scopus":                        # your home WiFi SSID
          password: "your_wifi_password"
```

```bash
sudo chmod 600 /etc/netplan/50-cloud-init.yaml
sudo netplan apply
sleep 15

# Must succeed before continuing
ping -I wlan0 -c 4 8.8.8.8
```

---

## Phase 4 — Static IP for wlan1 (TP-Link dongle)

```bash
sudo nano /etc/systemd/network/20-wlan1.network
```

```ini
[Match]
Name=wlan1

[Network]
Address=192.168.50.1/24

[Link]
RequiredForOnline=no
```

```bash
sudo systemctl restart systemd-networkd
ip addr show wlan1    # must show 192.168.50.1
```

---

## Phase 5 — Configure hostapd (Optimized for RTL8188EUS)

```bash
sudo nano /etc/hostapd/hostapd.conf
```

```ini
# Interface
interface=wlan1
driver=nl80211

# Your extended WiFi name and password
ssid=scopus_extended
wpa_passphrase=YourStrongPassword123

# Country (important for legal TX power in India)
country_code=IN
ieee80211d=1

# Radio — 2.4GHz with 802.11n enabled
hw_mode=g
channel=6                     # Change after checking least congested channel
ieee80211n=1
wmm_enabled=1

# RTL8188EUS supported capabilities
ht_capab=[SHORT-GI-20][SHORT-GI-40][HT40+]

# Security — WPA2 only (faster than TKIP)
auth_algs=1
wpa=2
wpa_key_mgmt=WPA-PSK
wpa_pairwise=CCMP
rsn_pairwise=CCMP

# Stability settings
beacon_int=100
dtim_period=2
max_num_sta=20
rts_threshold=2347
fragm_threshold=2346
```

```bash
# Point hostapd to config
sudo nano /etc/default/hostapd
```

Set this line:
```
DAEMON_CONF="/etc/hostapd/hostapd.conf"
```

---

## Phase 6 — Configure dnsmasq (DHCP for clients)

```bash
sudo mv /etc/dnsmasq.conf /etc/dnsmasq.conf.bak
sudo nano /etc/dnsmasq.conf
```

```ini
# Only serve the AP interface
interface=wlan1
bind-interfaces
except-interface=wlan0

# Give devices IPs in this range
dhcp-range=192.168.50.10,192.168.50.100,255.255.255.0,24h

# Fast DNS servers
server=1.1.1.1
server=8.8.8.8
no-resolv

# Tell clients: gateway = Pi, DNS = fast servers
dhcp-option=3,192.168.50.1
dhcp-option=6,1.1.1.1,8.8.8.8

# Cache DNS for speed
cache-size=1000
```

---

## Phase 7 — IP Forwarding + NAT

```bash
# Enable forwarding permanently
sudo nano /etc/sysctl.d/99-extender.conf
```

```ini
net.ipv4.ip_forward=1
```

```bash
sudo sysctl --system

# NAT rules
sudo iptables -t nat -A POSTROUTING -o wlan0 -j MASQUERADE
sudo iptables -A FORWARD -i wlan1 -o wlan0 -j ACCEPT
sudo iptables -A FORWARD -i wlan0 -o wlan1 -m state --state RELATED,ESTABLISHED -j ACCEPT

# Save rules permanently (restored automatically on every boot)
sudo apt install -y iptables-persistent
sudo netfilter-persistent save
```

---

## Phase 8 — Speed Optimization (sysctl tuning + BBR)

```bash
sudo nano /etc/sysctl.d/99-speed.conf
```

```ini
# BBR congestion control — significantly better throughput
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr

# Larger network buffers
net.core.rmem_max=16777216
net.core.wmem_max=16777216
net.ipv4.tcp_rmem=4096 87380 16777216
net.ipv4.tcp_wmem=4096 65536 16777216

# Reduce latency
net.core.netdev_max_backlog=5000
net.ipv4.tcp_fastopen=3
net.ipv4.tcp_mtu_probing=1
```

```bash
sudo sysctl --system

# Verify BBR is active
sysctl net.ipv4.tcp_congestion_control
# Should print: net.ipv4.tcp_congestion_control = bbr
```

---

## Phase 9 — Auto-start After Every Reboot (Critical)

**Fix hostapd startup order** — wait for wlan1 to exist:

```bash
sudo mkdir -p /etc/systemd/system/hostapd.service.d/
sudo nano /etc/systemd/system/hostapd.service.d/override.conf
```

```ini
[Unit]
After=sys-subsystem-net-devices-wlan1.device network-online.target
Wants=sys-subsystem-net-devices-wlan1.device network-online.target

[Service]
Restart=always
RestartSec=5s
```

**Fix dnsmasq startup order** — wait for hostapd:

```bash
sudo mkdir -p /etc/systemd/system/dnsmasq.service.d/
sudo nano /etc/systemd/system/dnsmasq.service.d/override.conf
```

```ini
[Unit]
After=hostapd.service network-online.target

[Service]
Restart=always
RestartSec=5s
```

**Enable all required services:**

```bash
sudo systemctl daemon-reload
sudo systemctl unmask hostapd
sudo systemctl enable hostapd dnsmasq netfilter-persistent systemd-networkd-wait-online
sudo systemctl start hostapd dnsmasq
```

---

## Phase 10 — Watchdog (Auto-recover if anything drops)

```bash
sudo nano /usr/local/bin/wifi-watchdog.sh
```

```bash
#!/bin/bash
LOG="/var/log/wifi-watchdog.log"
PING_HOST="1.1.1.1"

# Check internet via wlan0
if ! ping -I wlan0 -c 3 -W 3 "$PING_HOST" &>/dev/null; then
    echo "$(date): wlan0 lost internet — restarting networkd" >> "$LOG"
    systemctl restart systemd-networkd
    sleep 15
    systemctl restart hostapd
    sleep 3
    systemctl restart dnsmasq
fi

# Check hostapd is running
if ! systemctl is-active --quiet hostapd; then
    echo "$(date): hostapd died — restarting" >> "$LOG"
    systemctl restart hostapd
    sleep 3
    systemctl restart dnsmasq
fi
```

```bash
sudo chmod +x /usr/local/bin/wifi-watchdog.sh

# Run every 3 minutes
sudo crontab -e
```

Add this line:
```
*/3 * * * * /usr/local/bin/wifi-watchdog.sh
```

---

## Final Reboot Test

```bash
sudo reboot
```

After ~90 seconds, SSH back in and verify:

```bash
# 1. Driver loaded correctly
lsmod | grep 8188                         # shows 8188eu

# 2. wlan0 has IP from home router
ip addr show wlan0                        # shows 192.x.x.x

# 3. wlan1 has static IP
ip addr show wlan1                        # shows 192.168.50.1

# 4. Both services running
sudo systemctl status hostapd             # active (running)
sudo systemctl status dnsmasq            # active (running)

# 5. Internet works
ping -I wlan0 -c 3 1.1.1.1              # 3 packets received

# 6. iptables rules are there
sudo iptables -t nat -L -v              # shows MASQUERADE rule

# 7. See clients when connected
sudo iw dev wlan1 station dump
```

---

## Boot Sequence (What happens after power outage)

```
Power returns → Pi boots
     ↓
Kernel loads → 8188eu module loads (DKMS) → wlan1 appears
     ↓
systemd-networkd → wlan0 connects to scopus → wlan1 gets 192.168.50.1
     ↓
netfilter-persistent → restores iptables NAT rules
     ↓
hostapd → waits for wlan1 → broadcasts "scopus_extended"
     ↓
dnsmasq → waits for hostapd → DHCP ready
     ↓
Watchdog → checks every 3 min → auto-heals if anything drops
     ↓
✅ Devices connect and get internet within ~90 seconds of Pi boot
```

---

## Speed Tips Summary

| Action | Speed gain |
|--------|-----------|
| Connect wlan0 to **5GHz** band | 🔥 Biggest improvement |
| Use channel **1, 6, or 11** (least congested) | Medium |
| Keep Pi **close to home router** | Medium |
| BBR congestion control (Phase 8) | Small–medium |
| WPA2-CCMP only (no TKIP) | Small |
| Use WiFi analyzer app to pick best channel | Small |

> Install **WiFi Analyzer** on Android to scan nearby networks and pick the least used channel (1, 6, or 11) for your `scopus_extended` AP.

# Ethernet → Pi → TP-Link WiFi (Best Setup!)

This is **far superior** to WiFi-to-WiFi. You should definitely do this!

## Why This is Much Better

```
❌ Old Way:
scopus ──(WiFi 2.4GHz)──► wlan0 ──► wlan1 (TP-Link) ──(WiFi 2.4GHz)──► Devices
         half bandwidth          two radios interfere

✅ New Way (Your idea):
scopus ──(Ethernet 1Gbps)──► eth0 ──► wlan1 (TP-Link) ──(WiFi 2.4GHz)──► Devices
         full gigabit              zero interference
```

| What improves | Why |
|---------------|-----|
| 🔥 Much faster uplink | Ethernet vs WiFi — no comparison |
| 🔥 Zero interference | wlan0 not used at all |
| 🔥 Stable connection | Ethernet never drops |
| 🔥 Simpler setup | One wireless interface only |
| 🔥 Lower latency | Wired uplink = instant |
| 🔥 Max client speed | All bandwidth goes to TP-Link clients |

> **The only bottleneck now is the TP-Link dongle itself** (N150 = ~40–60 Mbps real-world). Your ethernet uplink will never be the limiting factor.

---

## Full Setup Guide

### Phase 1 — TP-Link v2 Driver (Same as before)

```bash
sudo apt update && sudo apt full-upgrade -y
sudo apt install -y dkms git bc build-essential linux-headers-$(uname -r)

# Remove broken stock driver
sudo apt remove -y r8188eu-dkms 2>/dev/null || true

# Install AP-capable driver
git clone https://github.com/aircrack-ng/rtl8188eus /tmp/rtl8188eus
cd /tmp/rtl8188eus
sudo make dkms_install

# Blacklist stock driver permanently
echo "blacklist r8188eu" | sudo tee /etc/modprobe.d/blacklist-r8188eu.conf
echo "blacklist rtl8188eu" | sudo tee -a /etc/modprobe.d/blacklist-r8188eu.conf

sudo update-initramfs -u
sudo reboot
```

**After reboot — verify:**

```bash
lsmod | grep 8188           # must show 8188eu
sudo iw list | grep -A 10 "Supported interface modes"  # must show AP
ip link show | grep -E "eth0|wlan"  # eth0 and wlan1 must exist
```

---

### Phase 2 — Prevent USB Power Cutoff

```bash
sudo nano /etc/udev/rules.d/99-usb-power.rules
```

```
ACTION=="add", SUBSYSTEM=="usb", ATTR{idVendor}=="2357", ATTR{idProduct}=="010c", ATTR{power/autosuspend}="-1"
```

```bash
sudo udevadm control --reload-rules && sudo udevadm trigger
```

---

### Phase 3 — Configure eth0 (Ethernet from scopus router)

```bash
ls /etc/netplan/     # find your config filename
sudo nano /etc/netplan/50-cloud-init.yaml
```

```yaml
network:
  version: 2
  renderer: networkd
  ethernets:
    eth0:
      dhcp4: yes
      dhcp4-overrides:
        route-metric: 100
```

```bash
sudo chmod 600 /etc/netplan/50-cloud-init.yaml
sudo netplan apply
sleep 10

# Must work before you continue
ping -I eth0 -c 4 8.8.8.8
```

---

### Phase 4 — Static IP on wlan1 (TP-Link AP)

```bash
sudo nano /etc/systemd/network/20-wlan1.network
```

```ini
[Match]
Name=wlan1

[Network]
Address=192.168.50.1/24

[Link]
RequiredForOnline=no
```

```bash
sudo systemctl restart systemd-networkd
ip addr show wlan1      # must show 192.168.50.1
```

---

### Phase 5 — hostapd Config (Optimized for RTL8188EUS v2)

```bash
sudo nano /etc/hostapd/hostapd.conf
```

```ini
# Interface — only wlan1 (TP-Link dongle)
interface=wlan1
driver=nl80211

# Your broadcast WiFi name and password
ssid=MyHomeWiFi
wpa_passphrase=StrongPassword123

# Country code (Kerala, India)
country_code=IN
ieee80211d=1

# 2.4GHz with 802.11n (N150 max)
hw_mode=g
channel=6               # use WiFi Analyzer app to find best channel
ieee80211n=1
wmm_enabled=1

# RTL8188EUS-supported HT caps
ht_capab=[SHORT-GI-20][HT40+]

# WPA2 only — fastest and most secure
auth_algs=1
wpa=2
wpa_key_mgmt=WPA-PSK
wpa_pairwise=CCMP
rsn_pairwise=CCMP

# Stability
beacon_int=100
dtim_period=2
max_num_sta=20
```

```bash
sudo nano /etc/default/hostapd
```

Set:
```
DAEMON_CONF="/etc/hostapd/hostapd.conf"
```

---

### Phase 6 — dnsmasq (Gives IPs to WiFi clients)

```bash
sudo mv /etc/dnsmasq.conf /etc/dnsmasq.conf.bak
sudo nano /etc/dnsmasq.conf
```

```ini
interface=wlan1
bind-interfaces
except-interface=eth0

# IP range given to connected devices
dhcp-range=192.168.50.10,192.168.50.100,255.255.255.0,24h

# Fast public DNS
server=1.1.1.1
server=8.8.8.8
no-resolv

# Tell clients: Pi is the gateway and DNS
dhcp-option=3,192.168.50.1
dhcp-option=6,1.1.1.1,8.8.8.8

# DNS cache for speed
cache-size=1000
```

---

### Phase 7 — IP Forwarding + NAT

> **Key difference from WiFi-to-WiFi:** NAT routes `wlan1 → eth0` now, not `wlan1 → wlan0`

```bash
# Permanent forwarding
sudo nano /etc/sysctl.d/99-extender.conf
```

```ini
net.ipv4.ip_forward=1

# BBR for better speed
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr

# Bigger buffers
net.core.rmem_max=16777216
net.core.wmem_max=16777216
net.ipv4.tcp_rmem=4096 87380 16777216
net.ipv4.tcp_wmem=4096 65536 16777216
net.core.netdev_max_backlog=5000
net.ipv4.tcp_fastopen=3
net.ipv4.tcp_mtu_probing=1
```

```bash
sudo sysctl --system

# NAT — eth0 is the outgoing interface now!
sudo iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
sudo iptables -A FORWARD -i wlan1 -o eth0 -j ACCEPT
sudo iptables -A FORWARD -i eth0 -o wlan1 -m state --state RELATED,ESTABLISHED -j ACCEPT

# Save permanently
sudo apt install -y iptables-persistent
sudo netfilter-persistent save
```

---

### Phase 8 — Auto-start + Boot Recovery

```bash
# hostapd waits for wlan1 to exist
sudo mkdir -p /etc/systemd/system/hostapd.service.d/
sudo nano /etc/systemd/system/hostapd.service.d/override.conf
```

```ini
[Unit]
After=sys-subsystem-net-devices-wlan1.device network-online.target
Wants=sys-subsystem-net-devices-wlan1.device

[Service]
Restart=always
RestartSec=5s
```

```bash
# dnsmasq waits for hostapd
sudo mkdir -p /etc/systemd/system/dnsmasq.service.d/
sudo nano /etc/systemd/system/dnsmasq.service.d/override.conf
```

```ini
[Unit]
After=hostapd.service network-online.target

[Service]
Restart=always
RestartSec=5s
```

```bash
sudo systemctl daemon-reload
sudo systemctl unmask hostapd
sudo systemctl enable hostapd dnsmasq netfilter-persistent
sudo systemctl start hostapd dnsmasq
```

---

### Phase 9 — Watchdog for Auto-healing

```bash
sudo nano /usr/local/bin/wifi-watchdog.sh
```

```bash
#!/bin/bash
LOG="/var/log/wifi-watchdog.log"

# Check ethernet has internet
if ! ping -I eth0 -c 3 -W 3 1.1.1.1 &>/dev/null; then
    echo "$(date): eth0 lost internet" >> "$LOG"
    systemctl restart systemd-networkd
    sleep 10
fi

# Restart hostapd if dead
if ! systemctl is-active --quiet hostapd; then
    echo "$(date): hostapd died — restarting" >> "$LOG"
    systemctl restart hostapd
    sleep 5
    systemctl restart dnsmasq
fi

# Restart dnsmasq if dead
if ! systemctl is-active --quiet dnsmasq; then
    echo "$(date): dnsmasq died — restarting" >> "$LOG"
    systemctl restart dnsmasq
fi
```

```bash
sudo chmod +x /usr/local/bin/wifi-watchdog.sh

# Run every 2 minutes
sudo crontab -e
```

Add:
```
*/2 * * * * /usr/local/bin/wifi-watchdog.sh
```

---

## Final Reboot Test

```bash
sudo reboot
```

After ~60 seconds:

```bash
# Check everything
ip addr show eth0              # has IP from scopus router
ip addr show wlan1             # shows 192.168.50.1
systemctl status hostapd       # active (running)
systemctl status dnsmasq       # active (running)
ping -I eth0 -c 3 1.1.1.1    # internet works
iptables -t nat -L -v          # MASQUERADE rule on eth0
iw dev wlan1 info              # AP mode, SSID broadcasting
```

---

## Power Outage Recovery Flow

```
Power restored → Pi boots (~30 sec)
      ↓
eth0 → gets IP from scopus router via DHCP (~10 sec)
      ↓
wlan1 → gets 192.168.50.1 static IP (~5 sec)
      ↓
netfilter-persistent → restores iptables NAT rules instantly
      ↓
hostapd → detects wlan1 is ready → broadcasts WiFi (~5 sec)
      ↓
dnsmasq → starts DHCP server for clients (~2 sec)
      ↓
✅ Everything up in ~60 seconds automatically
      ↓
Watchdog → checks every 2 min → heals anything that missed startup
```

---

## This Setup vs WiFi-to-WiFi Comparison

| | Ethernet Setup ✅ | WiFi-to-WiFi ❌ |
|--|-------------------|-----------------|
| Uplink speed | 1 Gbps | ~150 Mbps |
| Uplink stability | Never drops | Can disconnect |
| Interference | Zero | High (both 2.4GHz) |
| Config complexity | Simple | Complex |
| After power cut | ~60 sec | ~90 sec |
| Real client speed | ~50 Mbps (dongle limit) | ~25 Mbps (halved) |

**Bottom line — your idea to use ethernet is 100% correct and the right way to do this.** The TP-Link dongle becomes a pure access point with zero uplink interference, giving your WiFi clients the full N150 speed instead of half.