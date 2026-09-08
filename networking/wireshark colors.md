 Wireshark uses color-coded rows to help you visually filter and understand vast amounts of network traffic at a glance. By default, colors correspond to specific protocols or packet behaviors, prioritizing network health and common traffic types. [1, 2]  
Default Packet Colors & Meanings 

• Black → Network Errors / Malformed Packets: Indicates severe issues like bad checksums, truncated packets, or corrupted data. 
• Red → TCP Problems: Highlights severe TCP issues such as connection resets (), retransmissions, or unreachable destinations. 
• Dark Gray → Session Setup/Teardown: Represents important TCP flags like , , or  used during the opening or closing of a connection. 
• Purple / Magenta → General TCP & Application Traffic: Often used for standard TCP traffic or application-layer data like HTTP. 
• Light Blue → UDP Traffic: Identifies connectionless datagrams, which typically include DNS queries, DHCP, and media streaming. 
• Green → HTTP Traffic: Highlights web traffic, making standard unencrypted browsing exchanges visually distinct. 
• Yellow / Orange → Network Control & Routing: Often marks ARP (Address Resolution Protocol), ICMP (like ping requests), or routing protocols. [2, 3, 4, 5]  

Expert Information Colors 
Wireshark also uses a color-coding scheme in the Expert Information window (found under Analyze &gt; Expert Information) to indicate the severity of an event: 

• Dark Blue (Chat): Normal workflow information (e.g., a TCP packet with the SYN flag set). 
• Cyan (Note): Notable events (e.g., encountering an HTTP 404 error). 
• Yellow (Warn): Warnings and unusual conditions. 
• Red (Error): Serious problems or application errors. [6]  

How to Customize Your View 
You aren't locked into these default colors. You can easily manage, create, or disable rules to fit your exact needs: 

1. Access Settings: Go to View &gt; Coloring Rules... in the top menu. 
2. Toggle On/Off: Uncheck a box to disable a rule, or check it to apply it to your capture. 
3. Add Custom Rules: Click the + (plus) icon to define your own specific display filters and set custom background or text colors. [2, 8, 9, 10, 11]  
