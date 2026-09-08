TLV (Type-Length-Value or Tag-Length-Value) is a flexible data encoding format used in networking protocols to pass structured information. It breaks data into three distinct fields: 

• Type: A unique code that identifies what the data represents. 
• Length: Indicates the size/length of the value in bytes. 
• Value: The actual data or payload itself. [4, 5, 6]  

Why is it used? 
Unlike rigid, fixed-header packet formats, TLV makes protocols highly extensible. If a network protocol needs to be updated, developers can simply introduce new "Type" codes without breaking the existing structure. Devices that do not recognize the new Type can safely ignore or skip the TLV block. [6, 7, 8, 9, 10]  
Common Networking Examples 
TLV is the underlying data structure for many foundational routing and discovery protocols: 

• IS-IS (Intermediate System-to-Intermediate System): This routing protocol is incredibly extensible because it uses TLVs to carry entirely new types of data (like IPv6 routing information) in the same packets used for basic routing. 
• CDP (Cisco Discovery Protocol): Uses TLVs to transmit device details like IP addresses, hardware platforms, and software versions between directly connected Cisco devices. 
• LLDP (Link Layer Discovery Protocol): The vendor-neutral equivalent of CDP. It uses TLVs to advertise system capabilities and identity across a local network. 
• BGP (Border Gateway Protocol): Utilizes TLVs to carry various path attributes like Network Layer Reachability Information (NLRI). [6]  



