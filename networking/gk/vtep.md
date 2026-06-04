A VTEP (VXLAN Tunnel Endpoint) is a networking component used in VXLAN (Virtual Extensible LAN) environments. Its primary job is to encapsulate and de-encapsulate data packets, acting as the bridge between virtual overlay networks and underlying physical (underlay) networks. [1, 2, 3, 4, 5]  
How a VTEP Works 
When a virtual machine or container sends a standard Layer 2 Ethernet frame, the VTEP processes it through three main steps: 

1. Encapsulation: The originating VTEP wraps the original Ethernet frame in a new VXLAN header, along with outer IP and UDP headers, so it can travel across a Layer 3 (IP) network. 
2. Transport: The packet is routed across the physical underlay network to the destination VTEP. 
3. De-encapsulation: The receiving VTEP strips away the outer headers and delivers the original Ethernet frame to the destination device. [2, 3, 7]  

Types of VTEPs 

• Hardware VTEPs: Physical network switches or routers (typically Top-of-Rack data center switches) that handle encapsulation. 
• Software VTEPs: Virtual switches or hypervisors (such as VMware vSwitch or Open vSwitch) running directly on host servers. [3, 6, 8, 9, 10]  

Key Concepts 

• VXLAN Network Identifier (VNI): A unique tag (similar to a VLAN ID) used by VTEPs to map traffic to the correct virtual network segment. 
• Control Planes: VTEPs often use protocols like BGP EVPN to learn the MAC addresses and IP addresses of remote endpoints, allowing them to route traffic efficiently without flooding the network. [7, 11]  

To read more about implementation and architecture, check out NetworkLessons' VXLAN Introduction or HPE's VXLAN Overview. 


