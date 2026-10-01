Resource Public Key Infrastructure (RPKI) is a cryptographic security framework that validates BGP routing on the internet. It allows legitimate IP address owners to cryptographically authorize exactly which Autonomous System Numbers (ASNs) can announce their IP prefixes, preventing route hijacks and leaks. [1, 2, 3, 4]  
How RPKI Works 
RPKI is built on three core mechanisms that ensure BGP announcements are authentic: 

1. Digital Certificates: Regional Internet Registries (RIRs) issue digital certificates that bind IP address blocks and ASNs to their rightful owners, mirroring the hierarchical structure of internet resource allocation. 
2. Route Origin Authorization (ROA): Resource holders use their private keys to create signed statements called ROAs. A ROA explicitly states: "ASN X is authorized to originate IP Prefix Y." 
3. Validation: Network operators run Relying Party (RP) software to download these cryptographically signed objects from global repositories. Their routers then use BGP Route Origin Validation (ROV) to compare incoming BGP announcements against this validated data. [1, 2]  

Why RPKI Matters 
Historically, the internet's routing system operated entirely on trust, meaning any network could falsely claim to own another network's IP addresses. RPKI mitigates these vulnerabilities by categorizing BGP routes into three states: 

• Valid: The announcement matches a valid ROA. 
• Invalid: The announcement conflicts with an existing ROA (e.g., an unauthorized ASN is advertising the prefix). Routers typically drop these routes. 
• NotFound: No ROA exists for the prefix, meaning the route is neither explicitly authorized nor explicitly denied. [2, 9, 10]  

Deployment and Tools 
For network administrators looking to secure their BGP infrastructure, RPKI is supported by several open-source validators and tools: 

• Validators: Popular RP software stacks include rpki-client, RIPE NCC Validator, and FORT Validator. 
• Management: You can create and manage your ROAs through your RIR portal, such as the ARIN RPKI Portal. [11, 12, 13, 14, 15]  

For a complete beginner-friendly breakdown of how RPKI secures BGP routing against malicious attacks and how to establish it for your own network: 



