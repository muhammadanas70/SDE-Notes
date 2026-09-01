# Bug Bounty & Application Security Engineering: An In-Depth Mental Model

> **Purpose:** Learn how common high-impact vulnerabilities work *under the hood*, how vulnerable implementations arise, how secure implementations are designed, and how to reason from a bug to its real security impact.
>
> **Safety:** The examples in this guide are intentionally simplified and are meant for local labs, code review, secure design, and authorized bug-bounty testing only. Do not test systems you do not own or lack explicit authorization to assess. Where an exploit is shown, it stops at a controlled demonstration rather than providing operational instructions for abusing a real target.
>
> **Format:** Markdown (`.md`), with ASCII architecture/data-flow diagrams only. No SVG diagrams.

---

## Table of Contents

1. [The Core Mental Model](#1-the-core-mental-model)
2. [How Modern Applications Actually Work](#2-how-modern-applications-actually-work)
3. [Security Boundaries and Trust Zones](#3-security-boundaries-and-trust-zones)
4. [Threat Modeling](#4-threat-modeling)
5. [The Attacker's Reasoning Loop](#5-the-attackers-reasoning-loop)
6. [Authentication](#6-authentication)
7. [Authorization and IDOR/BOLA](#7-authorization-and-idorbola)
8. [Session Security](#8-session-security)
9. [Account Takeover](#9-account-takeover)
10. [Password Reset and Recovery Flows](#10-password-reset-and-recovery-flows)
11. [MFA and MFA Bypass](#11-mfa-and-mfa-bypass)
12. [OAuth 2.0 / OpenID Connect](#12-oauth-20--openid-connect)
13. [Business Logic Vulnerabilities](#13-business-logic-vulnerabilities)
14. [Race Conditions and TOCTOU](#14-race-conditions-and-toctou)
15. [Injection: The General Principle](#15-injection-the-general-principle)
16. [SQL Injection](#16-sql-injection)
17. [Command Injection and RCE](#17-command-injection-and-rce)
18. [Server-Side Request Forgery (SSRF)](#18-server-side-request-forgery-ssrf)
19. [File Upload Vulnerabilities](#19-file-upload-vulnerabilities)
20. [Path Traversal and File Access](#20-path-traversal-and-file-access)
21. [Cross-Site Scripting (XSS)](#21-cross-site-scripting-xss)
22. [CSRF](#22-csrf)
23. [CORS and Cross-Origin Trust](#23-cors-and-cross-origin-trust)
24. [Prototype Pollution and Object Confusion](#24-prototype-pollution-and-object-confusion)
25. [Deserialization and Dangerous Parsing](#25-deserialization-and-dangerous-parsing)
26. [API Security](#26-api-security)
27. [GraphQL Security](#27-graphql-security)
28. [Mass Assignment and Property-Level Authorization](#28-mass-assignment-and-property-level-authorization)
29. [JWT and Token Security](#29-jwt-and-token-security)
30. [Webhooks and Signature Verification](#30-webhooks-and-signature-verification)
31. [Cloud Metadata and IAM Security](#31-cloud-metadata-and-iam-security)
32. [SSRF-to-Cloud Attack Chains](#32-ssrf-to-cloud-attack-chains)
33. [Microservices and Service-to-Service Trust](#33-microservices-and-service-to-service-trust)
34. [Message Queues and Event-Driven Systems](#34-message-queues-and-event-driven-systems)
35. [Caching and Cache Poisoning](#35-caching-and-cache-poisoning)
36. [SaaS Multi-Tenancy](#36-saas-multi-tenancy)
37. [File/Object Storage Security](#37-fileobject-storage-security)
38. [Secrets Management](#38-secrets-management)
39. [Cryptography: What Secure Code Really Means](#39-cryptography-what-secure-code-really-means)
40. [Smart Contracts and DeFi Security](#40-smart-contracts-and-defi-security)
41. [Oracles, Pricing, and Economic Invariants](#41-oracles-pricing-and-economic-invariants)
42. [Bridges and Cross-Chain Systems](#42-bridges-and-cross-chain-systems)
43. [Governance and Privileged Roles](#43-governance-and-privileged-roles)
44. [Browser and Mobile Security Boundaries](#44-browser-and-mobile-security-boundaries)
45. [Sandbox Escapes and Kernel Security: Conceptual Model](#45-sandbox-escapes-and-kernel-security-conceptual-model)
46. [Exploit Chains: From Primitive to Critical Impact](#46-exploit-chains-from-primitive-to-critical-impact)
47. [Secure Architecture Patterns](#47-secure-architecture-patterns)
48. [Secure Coding Patterns](#48-secure-coding-patterns)
49. [Testing Strategy](#49-testing-strategy)
50. [How to Read a Codebase Like a Security Researcher](#50-how-to-read-a-codebase-like-a-security-researcher)
51. [How to Build a Mental Model Efficiently](#51-how-to-build-a-mental-model-efficiently)
52. [A Practical Local Lab](#52-a-practical-local-lab)
53. [Bug Bounty Reporting and Severity Reasoning](#53-bug-bounty-reporting-and-severity-reasoning)
54. [Common False Positives and Dead Ends](#54-common-false-positives-and-dead-ends)
55. [Security Review Checklist](#55-security-review-checklist)
56. [Further Learning Roadmap](#56-further-learning-roadmap)

---

# 1. The Core Mental Model

The single most useful mental model is:

> **Security failure = an attacker can cross a boundary they should not be able to cross, or cause a trusted component to perform an action outside the intended policy.**

A vulnerability is therefore not merely a weird input. It is a mismatch between:

- what the system **believes** about the request,
- what the request **actually controls**, and
- what the system **allows** as a consequence.

A useful equation is:

```text
Security impact
  = attacker-controlled input
  × trust boundary crossed
  × authority gained
  × data/assets reached
  × repeatability / scale
  × reliability
```

A second model is even more useful during code review:

```text
SOURCE → TRANSFORMATION → SINK
   |           |             |
   |           |             +-- dangerous operation
   |           +---------------- validation / encoding / normalization
   +---------------------------- attacker-controlled or untrusted data
```

Examples:

```text
HTTP parameter → string concatenation → SQL query
                 ^ missing parameterization

URL parameter → server-side fetch   → internal service
               ^ missing destination policy

request JSON → object merge         → privileged field update
               ^ missing property allowlist

user cookie → session lookup         → account identity
              ^ session fixation / weak invalidation
```

The most productive security researchers repeatedly ask:

1. **Who controls this value?**
2. **What assumptions are made about it?**
3. **Where does it travel?**
4. **Which component interprets it?**
5. **What authority does that component have?**
6. **What security boundary is crossed?**
7. **What happens if the assumption is false?**
8. **Can the result be repeated, automated, or chained?**

---

# 2. How Modern Applications Actually Work

A typical internet application looks roughly like this:

```text
                         INTERNET
                            |
                            v
                    +---------------+
                    | DNS / CDN / WAF |
                    +-------+-------+
                            |
                            v
                  +--------------------+
                  | Load Balancer /     |
                  | Reverse Proxy       |
                  +---------+----------+
                            |
             +--------------+--------------+
             |                             |
             v                             v
      +-------------+               +-------------+
      | Web / API   |               | Static      |
      | Application |               | Assets      |
      +------+------+               +-------------+
             |
      +------+--------------------------+
      |              |                 |
      v              v                 v
+-----------+  +-----------+     +------------+
| Auth /    |  | Business  |     | Background |
| Identity  |  | Services  |     | Workers    |
+-----+-----+  +-----+-----+     +------+-----+
      |              |                  |
      +-------+------+---------+--------+
              |                |
              v                v
        +-----------+    +-----------+
        | Database  |    | Object    |
        | / Cache   |    | Storage   |
        +-----------+    +-----------+
              |
              v
        +-----------+
        | Event Bus  |
        | / Queues   |
        +-----------+
              |
              v
        +--------------------+
        | External APIs      |
        | Cloud Services     |
        +--------------------+
```

A request often crosses many trust boundaries:

```text
Browser
  |
  | attacker controls: URL, body, headers, cookies, timing
  v
CDN / Proxy
  |
  | may normalize / cache / redirect
  v
API Gateway
  |
  | authentication, rate limits
  v
Application
  |
  | authorization + business logic
  v
Database / Queue / Storage
  |
  | system identity may have high privileges
  v
External service / cloud control plane
```

A vulnerability can occur at **any boundary**.

The important shift in thinking is:

> Do not think of a web application as “a website.” Think of it as a distributed state machine that converts untrusted messages into privileged side effects.

---

# 3. Security Boundaries and Trust Zones

Typical trust zones:

```text
+------------------------------------------------------+
| Untrusted Internet                                  |
|                                                      |
|  +-------------------+                               |
|  | Attacker Browser  |                               |
|  +-------------------+                               |
+-------------------+----------------------------------+
                    |
                    v
+------------------------------------------------------+
| Edge                                                 |
| CDN / WAF / Reverse Proxy                            |
+-------------------+----------------------------------+
                    |
                    v
+------------------------------------------------------+
| Application Trust Zone                               |
|                                                      |
|  API → Auth → Business Logic → Data Access           |
+-------------------+----------------------------------+
                    |
                    v
+------------------------------------------------------+
| Privileged Data Zone                                 |
|                                                      |
| Databases | Object Storage | Message Brokers        |
+-------------------+----------------------------------+
                    |
                    v
+------------------------------------------------------+
| Infrastructure Control Plane                         |
|                                                      |
| Cloud IAM | Metadata | Deployment | Secrets         |
+------------------------------------------------------+
```

The strongest bugs often occur when a system mistakenly treats one zone as another.

Examples:

- user-controlled URL treated as a trusted internal destination;
- tenant ID treated as trusted because it came from a JWT claim;
- webhook body treated as authentic because it came from a known IP range;
- JSON field treated as harmless because the UI does not expose it;
- object identifier treated as authorized because authentication succeeded;
- cloud role treated as safe because “only the backend can call it.”

---

# 4. Threat Modeling

Threat modeling is structured curiosity.

A simple model:

```text
Assets
  ↓
Actors
  ↓
Entry Points
  ↓
Trust Boundaries
  ↓
Security Controls
  ↓
Failure Modes
  ↓
Impact
```

## 4.1 Assets

Identify things worth protecting:

- accounts;
- passwords and reset tokens;
- API keys;
- session tokens;
- private data;
- financial balances;
- orders;
- cloud credentials;
- source code;
- infrastructure control;
- cryptographic keys;
- smart-contract funds.

## 4.2 Actors

Define capabilities:

```text
Anonymous user
Authenticated standard user
Tenant administrator
Organization administrator
Support operator
Service account
Deployment identity
Cloud administrator
```

Then ask whether a lower-privileged actor can influence a higher-privileged action.

## 4.3 Entry points

Examples:

```text
GET /users/{id}
POST /orders
POST /password/reset
POST /oauth/callback
POST /webhooks/provider
POST /admin/export
PUT /profile
POST /upload
POST /proxy/fetch
```

## 4.4 Threat model output

A useful table:

| Entry point | Caller | Data controlled | Sensitive action | Main question |
|---|---|---|---|---|
| `/users/{id}` | user | object ID | read account | Is object access authorized? |
| `/orders/{id}/refund` | user/support | order ID | money movement | Who is allowed to refund? |
| `/proxy/fetch` | user | URL | server network access | Which destinations are trusted? |
| `/upload` | user | file | persistent storage/rendering | Is the content inert? |
| `/admin/export` | admin | filters | bulk data access | Can lower privilege reach it? |

---

# 5. The Attacker's Reasoning Loop

A strong bug bounty methodology is not “try payloads until something breaks.”

Use this loop:

```text
               +------------------+
               | Map functionality|
               +--------+---------+
                        |
                        v
               +------------------+
               | Identify trust   |
               | assumptions      |
               +--------+---------+
                        |
                        v
               +------------------+
               | Break one        |
               | assumption       |
               +--------+---------+
                        |
                        v
               +------------------+
               | Observe effect   |
               +--------+---------+
                        |
                        v
               +------------------+
               | Prove impact     |
               +--------+---------+
                        |
                        v
               +------------------+
               | Can it chain?    |
               +--------+---------+
                        |
                        +-------> repeat
```

The crucial stage is **assumption discovery**.

Instead of asking:

> “Does this endpoint have IDOR?”

Ask:

> “What exact property does the server use to decide whether object X belongs to actor Y?”

That question leads directly to implementation details.

---

# 6. Authentication

Authentication answers:

> “Who are you?”

Authorization answers:

> “What are you allowed to do?”

Never mix them.

## 6.1 Vulnerable authentication pattern

```python
# Vulnerable: demonstration only

def login(request, db):
    username = request.form["username"]
    password = request.form["password"]

    user = db.query_one(
        f"SELECT * FROM users WHERE username = '{username}' "
        f"AND password = '{password}'"
    )

    if user:
        return {"ok": True, "user_id": user["id"]}

    return {"ok": False}, 401
```

Problems:

1. SQL query concatenation enables SQL injection.
2. Passwords appear to be stored directly rather than using a password hash.
3. No rate limiting.
4. No account enumeration controls.
5. No MFA or additional risk controls.
6. Authentication state is returned without a secure session design.

## 6.2 Secure authentication structure

```python
from werkzeug.security import check_password_hash


def login(request, db, session_store):
    username = request.form.get("username", "")
    password = request.form.get("password", "")

    user = db.fetch_one(
        "SELECT id, password_hash, disabled FROM users WHERE username = %s",
        (username,)
    )

    # Use the same externally visible failure behavior for unknown/invalid users.
    if not user or user["disabled"]:
        return {"ok": False}, 401

    if not check_password_hash(user["password_hash"], password):
        return {"ok": False}, 401

    session_id = session_store.create(
        user_id=user["id"],
        rotate=True,
        ttl_seconds=3600,
    )

    response = {"ok": True}
    response_headers = {
        "Set-Cookie": (
            f"session={session_id}; "
            "HttpOnly; Secure; SameSite=Lax; Path=/"
        )
    }
    return response, 200, response_headers
```

The important architecture is:

```text
password
   |
   v
slow password hash verifier
   |
   v
identity established
   |
   v
new server-side session ID
   |
   v
authorization checks on every sensitive action
```

### Password storage

Never use:

```text
MD5(password)
SHA1(password)
SHA256(password)
```

Use a password hashing construction designed for passwords, such as Argon2id, scrypt, or bcrypt, with an appropriate configuration and independent password policy.

The security property you want is:

```text
attacker obtains database
        |
        v
cannot cheaply test billions of guesses per second
```

---

# 7. Authorization and IDOR/BOLA

This is among the most important topics in practical web security.

## 7.1 The vulnerability

Suppose the endpoint is:

```text
GET /api/invoices/1234
```

A vulnerable implementation might do:

```python
def get_invoice(request, invoice_id, db):
    require_login(request)

    invoice = db.fetch_one(
        "SELECT * FROM invoices WHERE id = %s",
        (invoice_id,)
    )

    return invoice
```

Authentication is present.
Authorization is absent.

The real security invariant should be:

```text
Can the current principal access invoice 1234?
```

not merely:

```text
Is invoice 1234 a real object?
```

## 7.2 Secure implementation

```python
def get_invoice(request, invoice_id, db):
    principal = require_login(request)

    invoice = db.fetch_one(
        """
        SELECT id, amount, created_at, status
        FROM invoices
        WHERE id = %s
          AND owner_user_id = %s
        """,
        (invoice_id, principal.user_id),
    )

    if invoice is None:
        # Deliberately avoid revealing whether another user's object exists.
        raise NotFound()

    return invoice
```

For organizations:

```python
def get_project_member(request, org_id, member_id, db):
    principal = require_login(request)

    allowed = db.fetch_value(
        """
        SELECT 1
        FROM organization_members
        WHERE organization_id = %s
          AND user_id = %s
          AND revoked_at IS NULL
        """,
        (org_id, principal.user_id),
    )

    if not allowed:
        raise Forbidden()

    return db.fetch_one(
        """
        SELECT id, display_name, role
        FROM organization_members
        WHERE organization_id = %s
          AND user_id = %s
        """,
        (org_id, member_id),
    )
```

## 7.3 Why IDOR is really an authorization bug

The identifier is not the problem.

This is safe:

```text
/invoices/1234
```

if the server verifies ownership or permission.

The vulnerable condition is:

```text
authentication != authorization
```

## 7.4 BOLA in APIs

The modern API term is often **Broken Object Level Authorization (BOLA)**.

A security review should examine every object identifier:

```text
user_id
account_id
invoice_id
order_id
team_id
project_id
file_id
document_id
payment_id
wallet_id
```

For every ID:

```text
Can I name an object?
    ↓
Can I read it?
    ↓
Can I modify it?
    ↓
Can I delete it?
    ↓
Can I trigger a side effect on it?
```

The highest impact often occurs on the last two questions.

---

# 8. Session Security

A session is the bridge between authentication and future requests.

Typical architecture:

```text
login
  |
  v
identity verified
  |
  v
random session identifier
  |
  +----------------------+
  |                      |
  v                      v
browser cookie       server-side store
                       |
                       v
                  user_id / roles
```

## 8.1 Properties of a strong session ID

It should be:

- unpredictable;
- high entropy;
- short-lived where appropriate;
- invalidated at logout where policy requires;
- rotated after authentication or privilege transitions;
- protected by secure cookie attributes;
- bound to server-side session state or securely signed token state.

## 8.2 Session fixation

Conceptually:

```text
attacker obtains/chooses session identifier
              |
              v
victim authenticates in that session
              |
              v
session now represents victim
              |
              v
attacker reuses same identifier
```

A robust system rotates the session identifier after successful authentication:

```python
session.rotate_identifier()
session.user_id = user.id
session.authenticated_at = now()
```

---

# 9. Account Takeover

Account takeover is an **impact class**, not one vulnerability.

A useful chain model:

```text
Primitive
   |
   +--> password reset weakness
   +--> session theft
   +--> OAuth token confusion
   +--> MFA recovery weakness
   +--> authorization bypass
   +--> credential exposure
   |
   v
Identity substitution
   |
   v
Victim account controlled by attacker
   |
   +--> private data
   +--> payment actions
   +--> API keys
   +--> organization access
   +--> admin privileges
```

The security researcher should always ask:

> “What is the minimum primitive needed to cross the identity boundary?”

For defensive review, enumerate every authentication transition:

```text
anonymous → account
account → verified account
account → MFA enabled
MFA → recovered account
user → admin
user → organization owner
```

Each transition needs explicit policy.

---

# 10. Password Reset and Recovery Flows

Password-reset endpoints are high-value because they modify identity credentials.

## Vulnerable pattern

```python
# Vulnerable conceptual example

def reset_password(request, db):
    email = request.form["email"]
    user = db.fetch_user_by_email(email)

    token = base64.urlsafe_b64encode(
        f"{user.id}:{user.email}".encode()
    ).decode()

    send_email(user.email, f"https://app/reset?token={token}")
```

Problems:

- token is deterministic;
- token may be forgeable;
- token contains identity rather than proof;
- no expiration shown;
- no one-time-use guarantee;
- potentially reveals account existence.

## Secure token design

```python
import secrets
from datetime import timedelta


def issue_reset_token(user, token_store, mailer):
    raw = secrets.token_urlsafe(32)

    token_store.insert(
        token_hash=sha256(raw.encode()).hexdigest(),
        user_id=user.id,
        expires_at=utcnow() + timedelta(minutes=20),
        used=False,
    )

    mailer.send_reset_link(user.email, raw)
```

The server stores a hash of the random token, not the raw token.

Verification:

```python
def consume_reset_token(raw, token_store):
    digest = sha256(raw.encode()).hexdigest()

    record = token_store.get_for_update(digest)

    if not record:
        raise InvalidToken()
    if record.used:
        raise InvalidToken()
    if record.expires_at <= utcnow():
        raise InvalidToken()

    token_store.mark_used(record.id)
    return record.user_id
```

The important invariants:

```text
unpredictable
+ short-lived
+ one-time-use
+ server-side state
+ explicit identity binding
```

---

# 11. MFA and MFA Bypass

Multi-factor authentication introduces additional state transitions.

Secure mental model:

```text
password correct
      |
      v
MFA challenge issued
      |
      v
challenge completed
      |
      v
session elevated to authenticated
```

A common architectural mistake is creating a full session before MFA is complete:

```text
password correct
      |
      v
FULL SESSION CREATED  <-- dangerous
      |
      v
MFA challenge
```

Instead:

```text
password correct
      |
      v
PARTIAL SESSION
      |
      v
MFA success
      |
      v
FULL SESSION CREATED
```

The partial session should not authorize sensitive actions.

## Recovery is part of MFA security

A strong MFA implementation can still be undermined by a weak recovery process.

Review:

- lost-device recovery;
- email-only fallback;
- support-assisted recovery;
- backup codes;
- trusted device enrollment;
- factor replacement;
- session revocation after factor changes.

The true security boundary is the **weakest authentication path**.

---

# 12. OAuth 2.0 / OpenID Connect

OAuth is an authorization protocol; OpenID Connect adds authentication semantics.

A simplified authorization-code architecture:

```text
+----------+                     +----------------+
| Browser  |                     | Authorization  |
| / Client |                     | Server         |
+----+-----+                     +-------+--------+
     |                                   |
     | authorize                         |
     +---------------------------------->
     |                                   |
     | redirect with code                |
     <-----------------------------------+
     |                                   |
     | exchange code                     |
     +---------------------------------->
     |                                   |
     | access/id token                   |
     <-----------------------------------+
     |
     v
+------------+
| Application|
+------------+
```

## Secure authorization-code flow

Use:

- exact redirect URIs;
- state validation;
- PKCE for public clients and where appropriate broadly;
- nonce validation for OIDC;
- issuer/audience validation;
- signature validation for ID tokens;
- strict client binding.

## Vulnerable redirect pattern

```python
# Dangerous conceptual example
redirect_uri = request.args["redirect_uri"]
return redirect(
    authorization_server.build_login_url(
        client_id=CLIENT_ID,
        redirect_uri=redirect_uri,
    )
)
```

The application must not accept arbitrary redirect destinations.

Secure pattern:

```python
ALLOWED_REDIRECTS = {
    "web": "https://app.example.test/oauth/callback",
}

redirect_uri = ALLOWED_REDIRECTS["web"]
```

For dynamic redirect handling, compare parsed URL components against a strict allowlist. Do not rely on string-prefix checks.

## Why state matters

Without request correlation:

```text
user begins login A
attacker controls authorization response B
application accepts response B as belonging to A
```

`state` binds the response to the login transaction.

---

# 13. Business Logic Vulnerabilities

Business logic bugs are often harder to detect than classic injection because the code can be perfectly syntactically safe.

Example:

```python
def apply_coupon(order, coupon):
    if coupon.valid:
        order.discount += coupon.amount
        return order
```

What is missing?

- Has the coupon already been used?
- Is it valid for this user?
- Is it valid for this product?
- Is it valid for this order total?
- Can it be applied twice?
- Can negative discounts occur?

Secure design makes invariants explicit:

```python

def apply_coupon(order, coupon, principal):
    if not coupon.is_active:
        raise InvalidCoupon()

    if not coupon.applies_to(order):
        raise InvalidCoupon()

    if coupon.used_by(principal.user_id):
        raise AlreadyUsed()

    discount = min(coupon.compute_discount(order), order.subtotal)

    order.apply_discount(discount)
    coupon.mark_used_by(principal.user_id)
```

But concurrency still matters. See race conditions.

## Think in invariants

Examples:

```text
balance >= 0
refund <= amount_paid
withdrawal <= available_balance
one-time coupon => at most one successful redemption
approved payment => cannot be altered by untrusted actor
role change => only authorized principal may cause it
```

Business logic security is largely **invariant security**.

---

# 14. Race Conditions and TOCTOU

TOCTOU means:

```text
Time Of Check
        ↓
        time gap
        ↓
Time Of Use
```

Vulnerable pattern:

```python
if account.balance >= amount:
    # another request may execute here
    account.balance -= amount
    save(account)
```

Two concurrent withdrawals can both pass the check.

Secure database transaction:

```sql
UPDATE accounts
SET balance = balance - :amount
WHERE id = :account_id
  AND balance >= :amount;
```

Then verify affected rows equals 1.

Even better, use an explicit transaction and appropriate isolation/locking strategy.

Conceptually:

```text
Request A ----check----use---->
Request B ------check----use-->
                 ^
                 |
           shared state
```

Security researchers should look for:

- counters;
- balances;
- inventory;
- one-time operations;
- coupon redemption;
- permission changes;
- invitation acceptance;
- file moves;
- state transitions.

---

# 15. Injection: The General Principle

Injection occurs when data crosses a boundary and becomes **instructions**.

General form:

```text
UNTRUSTED DATA
      |
      v
string / object composition
      |
      v
INTERPRETER
  SQL / shell / template / LDAP / HTML / expression / query engine
      |
      v
UNEXPECTED INSTRUCTIONS
```

The safest general principle is:

> **Keep data and code structurally separate.**

Examples:

| Interpreter | Secure separation |
|---|---|
| SQL | parameterized queries / prepared statements |
| HTML | context-aware output encoding + templating |
| Shell | avoid shell; use argument arrays / APIs |
| LDAP | parameterized APIs / escaping according to LDAP context |
| OS path | safe path APIs + allowlists + containment checks |
| NoSQL | typed query construction + operator controls |
| Template engine | no evaluation of user-controlled templates |

---

# 16. SQL Injection

## 16.1 Vulnerable code

```python
# Vulnerable
query = f"SELECT id, email FROM users WHERE email = '{email}'"
rows = db.execute(query)
```

The problem is structural: user data is inserted into SQL syntax.

## 16.2 Secure code

```python
query = "SELECT id, email FROM users WHERE email = %s"
rows = db.execute(query, (email,))
```

The database receives:

```text
SQL structure:   SELECT ... WHERE email = ?
Data:            attacker-controlled string
```

The database can distinguish them.

## 16.3 Dynamic sorting

A common mistake is parameterizing values but concatenating identifiers:

```python
# Still dangerous
query = f"SELECT * FROM users ORDER BY {sort}" 
```

Allowlist identifiers:

```python
SORTS = {
    "name": "display_name",
    "created": "created_at",
}

column = SORTS.get(sort)
if column is None:
    raise BadRequest()

query = f"SELECT * FROM users ORDER BY {column}"
```

The key lesson:

> Parameterization protects **values**, while identifiers often require **allowlisting**.

---

# 17. Command Injection and RCE

Command injection occurs when attacker-controlled data changes the command language interpreted by an operating-system shell or command processor.

## Vulnerable conceptual code

```python
# DO NOT use this pattern
import os

def convert(filename):
    os.system("convert " + filename + " output.png")
```

Secure alternative:

```python
import subprocess


def convert(filename):
    subprocess.run(
        ["convert", filename, "output.png"],
        check=True,
        shell=False,
    )
```

Even better, avoid general-purpose command execution when a library provides a safe API.

## RCE as a security boundary

A typical escalation is:

```text
Input
  |
  v
Injection primitive
  |
  v
Interpreter control
  |
  v
Code execution
  |
  v
Application identity
  |
  v
Filesystem / network / secrets
  |
  v
Potential broader compromise
```

The impact depends heavily on the identity running the process.

A process running as a minimally privileged user is much safer than one running with broad root/administrator privileges.

---

# 18. Server-Side Request Forgery (SSRF)

SSRF occurs when an attacker controls a server-side network request destination.

## Vulnerable pattern

```python
import requests


def fetch(request):
    url = request.args["url"]
    return requests.get(url, timeout=5).text
```

Architecture:

```text
Attacker
   |
   | URL
   v
Public API
   |
   | server-side HTTP request
   v
Internal network / external services
```

The key is that the server may have network access the attacker does not have.

## Why SSRF matters

Possible reachable zones include:

```text
localhost services
private RFC1918 networks
service discovery endpoints
internal admin APIs
cloud metadata services
management interfaces
```

## Secure architecture

Do not assume “filtering URLs” is easy.

Preferred pattern:

```text
User request
    |
    v
Resolve intended destination
    |
    v
Allowlisted destination registry
    |
    v
Outbound proxy / egress gateway
    |
    v
Approved targets only
```

Example allowlist:

```python
ALLOWED_HOSTS = {
    "api.partner.example",
    "images.partner.example",
}
```

Then resolve and validate carefully, including DNS rebinding considerations and IP-range policy. Network egress controls should enforce the same rule independently of application logic.

## Defense in depth

```text
Application allowlist
        +
DNS/IP policy
        +
Outbound network policy
        +
Metadata endpoint protections
        +
Minimal IAM permissions
```

Do not depend on one string check.

---

# 19. File Upload Vulnerabilities

File upload is a pipeline, not a single validation function.

```text
Browser
  |
  v
Upload API
  |
  +--> authentication
  +--> authorization
  +--> size/type validation
  +--> malware/content checks
  |
  v
Quarantine storage
  |
  v
Processing pipeline
  |
  +--> thumbnailer
  +--> OCR
  +--> antivirus
  +--> parser
  |
  v
Safe object storage
  |
  v
Delivery endpoint
```

## Vulnerable pattern

```python
filename = request.files["file"].filename
request.files["file"].save("/var/www/uploads/" + filename)
```

Problems:

- filename controls path;
- uploaded content may be executable;
- files may be served from an active web root;
- content type can be spoofed;
- no size limit;
- no malware scanning;
- no authorization model.

## Secure pattern

```python
import secrets
from pathlib import Path

UPLOAD_ROOT = Path("/srv/app/uploads")
ALLOWED_TYPES = {"image/jpeg", "image/png", "application/pdf"}


def store_upload(file, principal):
    if file.content_type not in ALLOWED_TYPES:
        raise BadRequest("unsupported type")

    data = file.read(MAX_UPLOAD_SIZE + 1)
    if len(data) > MAX_UPLOAD_SIZE:
        raise BadRequest("too large")

    # Validate content using trusted parsing, not only the client-supplied MIME type.
    validate_actual_content(data, file.content_type)

    object_key = f"{principal.user_id}/{secrets.token_hex(24)}"
    object_store.put(
        key=object_key,
        data=data,
        content_type=detected_content_type(data),
        private=True,
    )

    return object_key
```

The filename is metadata, not a filesystem instruction.

---

# 20. Path Traversal and File Access

Vulnerable:

```python
# Vulnerable
path = "/srv/files/" + request.args["name"]
return send_file(path)
```

The correct security invariant is:

```text
resolved requested path must remain inside approved root
```

Example:

```python
from pathlib import Path

ROOT = Path("/srv/files").resolve()


def safe_path(name: str) -> Path:
    candidate = (ROOT / name).resolve()

    try:
        candidate.relative_to(ROOT)
    except ValueError:
        raise Forbidden()

    return candidate
```

Still consider symlinks, mount points, race conditions, permissions, and whether the file itself is intended to be public.

A stronger model is to avoid mapping user-controlled names directly to filesystem paths at all:

```text
external object ID → database lookup → internal storage key
```

---

# 21. Cross-Site Scripting (XSS)

XSS means attacker-controlled data is interpreted as active browser content in a security context where it should remain data.

Three major families:

- reflected XSS;
- stored XSS;
- DOM-based XSS.

## Vulnerable server template

```html
<!-- Vulnerable conceptual template -->
<div>{{{user_display_name}}}</div>
```

If triple braces disable escaping, HTML becomes executable content.

## Secure template

```html
<div>{{user_display_name}}</div>
```

with framework-level HTML escaping enabled.

## Context matters

Encoding depends on the output context:

```text
HTML text
HTML attribute
JavaScript string
CSS value
URL
```

Do not apply one universal “sanitize” function everywhere.

## DOM XSS example

Vulnerable:

```javascript
const message = new URLSearchParams(location.search).get("message");
document.querySelector("#output").innerHTML = message;
```

Secure:

```javascript
const message = new URLSearchParams(location.search).get("message");
document.querySelector("#output").textContent = message ?? "";
```

For rich HTML, use a well-maintained HTML sanitizer with an appropriate allowlist.

## Strong browser defenses

Use a strong Content Security Policy where feasible, and avoid unsafe patterns such as arbitrary script execution and string-to-code evaluation.

---

# 22. CSRF

Cross-Site Request Forgery abuses the fact that browsers may automatically attach credentials such as cookies.

Example architecture:

```text
Victim browser
    |
    | authenticated cookie automatically attached
    v
Sensitive endpoint
    ^
    |
Attacker-controlled page triggers cross-origin request
```

## Vulnerable

```python
@app.post("/email/change")
def change_email(request):
    user = session_user(request)
    user.email = request.form["email"]
    save(user)
```

## Secure approaches

For cookie-authenticated state-changing requests, use an appropriate combination of:

- SameSite cookie settings;
- CSRF tokens;
- Origin/Referer validation where suitable;
- re-authentication for very sensitive actions;
- non-cookie authorization mechanisms where the architecture supports it.

Example synchronizer token:

```python
csrf = secrets.token_urlsafe(32)
session.csrf_token = csrf
```

Then:

```python
def validate_csrf(request, session):
    supplied = request.form.get("csrf_token")
    if not constant_time_compare(supplied, session.csrf_token):
        raise Forbidden()
```

CSRF is about **ambient authority**: credentials are silently attached to a request that the user did not intentionally initiate.

---

# 23. CORS and Cross-Origin Trust

CORS controls whether browser JavaScript from one origin may read responses from another origin.

CORS is not a general server-side access-control mechanism.

A dangerous pattern is reflecting arbitrary origins:

```python
response.headers["Access-Control-Allow-Origin"] = request.headers["Origin"]
response.headers["Access-Control-Allow-Credentials"] = "true"
```

This can be catastrophic if authenticated responses are exposed to attacker-controlled origins.

Secure pattern:

```python
ALLOWED_ORIGINS = {
    "https://app.example.com",
    "https://admin.example.com",
}

origin = request.headers.get("Origin")
if origin in ALLOWED_ORIGINS:
    response.headers["Access-Control-Allow-Origin"] = origin
    response.headers["Access-Control-Allow-Credentials"] = "true"
```

But CORS should complement, not replace, application authorization.

---

# 24. Prototype Pollution and Object Confusion

Prototype pollution is especially relevant in JavaScript applications that merge attacker-controlled object properties into shared object structures.

Conceptual vulnerable pattern:

```javascript
function deepMerge(target, source) {
  for (const key in source) {
    if (typeof source[key] === "object") {
      target[key] = target[key] || {};
      deepMerge(target[key], source[key]);
    } else {
      target[key] = source[key];
    }
  }
}
```

The danger arises when inherited/shared object structures can be modified or when special property names change interpreter behavior.

Secure patterns:

- use maintained merge utilities with safe-property handling;
- reject special object keys where appropriate;
- use `Object.create(null)` for dictionary-like structures where suitable;
- validate schemas before merging;
- avoid merging arbitrary user objects into security-sensitive configuration.

The deeper concept is **data structure confusion**: code assumes an object has a benign shape when the caller can influence that shape.

---

# 25. Deserialization and Dangerous Parsing

Deserialization is dangerous when data is interpreted as executable object graphs or privileged types.

The secure question is:

> Can untrusted data select a class, constructor, method, gadget, or resource behavior?

Avoid unsafe native object deserialization for untrusted input.

Prefer:

```text
wire data
  ↓
strict schema
  ↓
primitive values
  ↓
validated domain object
```

rather than:

```text
wire data
  ↓
magic serializer
  ↓
arbitrary object graph
  ↓
implicit behavior
```

Use JSON or typed schema systems where practical, and validate lengths, nesting depth, types, and allowed fields.

---

# 26. API Security

A modern API is a collection of state transitions.

For every endpoint, document:

```text
Authentication
Authorization
Input constraints
Output constraints
Rate limit
State changes
Side effects
Idempotency
Object ownership
Tenant boundaries
```

Example:

```text
POST /api/transfer

Principal: authenticated user

Inputs:
  destination_account_id
  amount
  currency
  idempotency_key

Security invariants:
  caller may spend from source account
  amount > 0
  amount <= available balance
  destination is valid
  currency is supported
  transfer is not replayed
  fraud/risk policy passes
```

A secure API implementation makes these invariants visible in code and tests.

---

# 27. GraphQL Security

GraphQL shifts some security concerns from route-level authorization toward field/object-level authorization.

Architecture:

```text
POST /graphql
     |
     v
Query parser
     |
     v
Resolver tree
  /    |    \\
User  Org  Billing
 |
 v
Data access
```

Potential problems:

- missing resolver-level authorization;
- excessive query depth;
- expensive nested queries;
- introspection exposure where not appropriate;
- object-level authorization gaps;
- mutation abuse;
- batching-based enumeration.

A resolver should enforce authorization based on the actual object and operation.

```python
@resolver
@require_permission("invoice.read")
def invoice(parent, info, invoice_id):
    principal = info.context.principal
    return invoice_service.get_for_principal(principal, invoice_id)
```

The resolver should not rely only on the UI hiding the field.

---

# 28. Mass Assignment and Property-Level Authorization

Consider:

```json
{
  "display_name": "Alice",
  "role": "admin"
}
```

Vulnerable code:

```python
user.update(request.json)
```

If `role` is security-sensitive, the API has turned a normal profile update into a possible privilege change.

Secure:

```python
ALLOWED_PROFILE_FIELDS = {"display_name", "timezone", "language"}

payload = request.json
safe = {
    key: payload[key]
    for key in ALLOWED_PROFILE_FIELDS
    if key in payload
}

user.update(safe)
```

For administrative properties, create separate commands:

```text
PATCH /profile
POST /admin/users/{id}/grant-role
POST /admin/users/{id}/disable
```

Separating security-sensitive mutations makes authorization easier to reason about.

---

# 29. JWT and Token Security

A JWT typically consists of:

```text
base64url(header) . base64url(payload) . base64url(signature)
```

The payload is generally readable; signing is not encryption.

Security checks should include:

```text
signature valid
issuer expected
audience expected
algorithm explicitly allowed
expiration valid
not-before valid where applicable
subject meaningful
key ID resolves to trusted key
```

Never treat decoded claims as trusted before verification.

## Vulnerable mental model

```python
claims = jwt.decode(token, options={"verify_signature": False})
user_id = claims["sub"]
```

## Secure model

```python
claims = jwt.decode(
    token,
    key=trusted_key,
    algorithms=["RS256"],
    audience="api",
    issuer="https://issuer.example.com",
)
```

But JWT itself does not solve authorization.

A valid token might prove:

```text
subject = user123
```

It does not automatically prove:

```text
user123 is allowed to modify resource XYZ
```

---

# 30. Webhooks and Signature Verification

Webhooks convert an external message into an internal side effect.

That is a trust boundary.

Secure flow:

```text
Provider
  |
  | body + signature + timestamp
  v
Webhook endpoint
  |
  +--> read raw body
  +--> verify signature
  +--> verify timestamp window
  +--> replay protection
  |
  v
trusted event
  |
  v
business action
```

## Common mistake

Parsing the body and then verifying a reconstructed representation.

Signature validation must normally operate on the exact bytes covered by the provider's signing specification.

Use constant-time comparison for MAC/signature-derived values where appropriate.

## Replay protection

A valid webhook captured earlier should not be usable indefinitely.

Common approach:

```text
signature = HMAC(secret, timestamp || raw_body)
```

and reject timestamps outside a reasonable window, with event IDs stored to prevent duplicate processing.

---

# 31. Cloud Metadata and IAM Security

Cloud environments contain a privileged control plane.

Application architecture:

```text
Application
   |
   | service identity
   v
Cloud IAM
   |
   +--> object storage
   +--> queues
   +--> databases
   +--> secrets
   +--> compute APIs
```

The dangerous assumption is:

> “The application server is trusted, so its cloud identity can be broad.”

Instead:

```text
frontend identity     → minimal read/write
worker identity       → queue + required storage
reporting identity    → read-only analytics
deployment identity  → deployment scope only
```

Least privilege is especially important because an RCE or SSRF vulnerability inherits the application's cloud identity.

---

# 32. SSRF-to-Cloud Attack Chains

This is a classic example of why vulnerability analysis must include downstream effects.

Conceptual chain:

```text
User-controlled URL
       |
       v
SSRF primitive
       |
       v
server can reach privileged internal endpoint
       |
       v
credential or privileged response exposed
       |
       v
cloud API access
       |
       v
resource manipulation / data access
```

The correct defense is not only “block one metadata hostname.”

Use:

```text
safe destination policy
+ network egress restrictions
+ metadata service protections
+ minimal workload IAM
+ secret rotation
+ monitoring
```

This is a general lesson:

> **A primitive inherits the privileges of the component through which it executes.**

---

# 33. Microservices and Service-to-Service Trust

A large application might look like:

```text
             +--------+
             |  API   |
             +---+----+
                 |
       +---------+----------+
       |         |          |
       v         v          v
   +-------+ +-------+ +-------+
   | users | | orders| |billing|
   +---+---+ +---+---+ +---+---+
       |         |          |
       +---------+----------+
                 |
                 v
             Event Bus
```

Common failure:

```text
API authenticates user
     |
     v
internal request to billing
     |
     v
billing trusts internal network
     |
     v
user-influenced data becomes privileged action
```

Better:

```text
API → authenticated service identity → billing
                              |
                              v
                       user context
                              |
                              v
                    billing authorization
```

Network location should not be the only trust signal.

---

# 34. Message Queues and Event-Driven Systems

Queues introduce asynchronous trust boundaries.

```text
Request
  |
  v
API → Queue → Worker → Database
              |
              v
        external side effects
```

Security concerns:

- unauthorized message publication;
- forged message schema;
- missing tenant identity validation;
- replay/duplicate processing;
- ordering assumptions;
- dead-letter queue exposure;
- sensitive data leakage in message bodies;
- confused deputy behavior.

Every message should have an explicit contract:

```json
{
  "event_id": "unique-id",
  "event_type": "invoice.created",
  "schema_version": 2,
  "actor": {"type": "user", "id": "..."},
  "tenant_id": "...",
  "occurred_at": "...",
  "payload": {}
}
```

Workers should verify the authority and context needed for the side effect rather than blindly trusting producer fields.

---

# 35. Caching and Cache Poisoning

Caching creates a second copy of application behavior.

```text
Client
  |
  v
CDN / cache
  |       \
  | HIT    \ MISS
  v          v
response   application
```

Security bugs can occur when:

- cache key omits security-relevant inputs;
- private content is cached as public;
- host/header normalization differs between cache and origin;
- query parameters are inconsistently interpreted;
- redirects or content types are cached unexpectedly.

The mental model:

> Is the cache deciding that two requests are equivalent when the application considers them different?

For authenticated responses, cache policy must be explicit.

---

# 36. SaaS Multi-Tenancy

A multi-tenant system might be:

```text
                    +----------------+
                    | Shared API     |
                    +--------+-------+
                             |
                +------------+------------+
                |            |            |
                v            v            v
             Tenant A     Tenant B     Tenant C
                |            |            |
                +------------+------------+
                             |
                             v
                       Shared DB
```

The core invariant:

```text
request.tenant_id == every resource's authorized tenant boundary
```

A common bad design:

```python
invoice = db.get_invoice(invoice_id)
```

A better design:

```python
invoice = db.get_invoice_for_tenant(
    invoice_id=invoice_id,
    tenant_id=principal.tenant_id,
)
```

For extreme isolation requirements, use row-level security, separate databases, separate credentials, or physical isolation based on risk.

---

# 37. File/Object Storage Security

Object storage is often accidentally exposed through:

- overly broad bucket policies;
- predictable object keys;
- permanent public URLs;
- confused tenant boundaries;
- server-side encryption/key policy mistakes;
- signed URL misuse.

A secure design usually separates:

```text
logical object ID
       |
       v
authorization check
       |
       v
private storage key
       |
       v
short-lived delivery URL / streaming response
```

Signed URLs should have:

- narrow scope;
- short expiry;
- correct content disposition;
- intended HTTP method;
- appropriate audience restrictions where supported.

---

# 38. Secrets Management

Avoid embedding secrets in:

- source code;
- mobile apps;
- browser JavaScript;
- container images;
- public repositories;
- logs.

Better architecture:

```text
Application
    |
    | workload identity
    v
Secrets manager
    |
    v
short-lived / narrowly scoped secret
```

A secret should have:

```text
purpose
owner
scope
rotation policy
expiration
access audit trail
```

Most importantly, assume secrets eventually leak and minimize their blast radius.

---

# 39. Cryptography: What Secure Code Really Means

Do not invent cryptography.

Use well-reviewed libraries and primitives.

Key concepts:

```text
Confidentiality → encryption
Integrity       → MAC / authenticated encryption / signatures
Authenticity    → signatures / authenticated channels
Randomness      → cryptographic CSPRNG
Password safety → password hashing
Key agreement   → established protocols
```

## Encryption vs hashing

Encryption is reversible with the key.

Hashing is one-way by design, although passwords should use dedicated password hashing rather than general cryptographic hashes.

## Authenticated encryption

Prefer schemes providing integrity and confidentiality together, such as an AEAD construction offered by a well-maintained library.

The dangerous pattern is:

```text
encrypt(message)
```

without authentication, because altered ciphertext may go undetected.

## Randomness

Use cryptographic randomness for:

- reset tokens;
- session identifiers;
- password-reset links;
- nonce values;
- API key generation.

Do not use predictable PRNGs for secrets.

---

# 40. Smart Contracts and DeFi Security

Smart contracts are different because state transitions can directly move assets.

A simplified architecture:

```text
User Wallet
    |
    v
Frontend / RPC
    |
    v
Smart Contract
    |
    +--> Token Contract
    |
    +--> Oracle
    |
    +--> Other Protocols
    |
    v
Permanent / composable state
```

The critical mental shift is:

> **Treat every public contract function as an attacker-controlled call.**

## 40.1 Vulnerable access control

```solidity
// Vulnerable conceptual example
function setTreasury(address newTreasury) external {
    treasury = newTreasury;
}
```

Any caller can change the treasury.

Secure:

```solidity
function setTreasury(address newTreasury) external onlyOwner {
    require(newTreasury != address(0), "invalid");
    treasury = newTreasury;
}
```

But `onlyOwner` itself must be securely initialized and governed.

## 40.2 Reentrancy

Vulnerable conceptual pattern:

```solidity
function withdraw(uint256 amount) external {
    require(balances[msg.sender] >= amount);

    (bool ok, ) = msg.sender.call{value: amount}("");
    require(ok);

    balances[msg.sender] -= amount;
}
```

The external call happens before state is updated.

Secure pattern:

```solidity
function withdraw(uint256 amount) external nonReentrant {
    uint256 balance = balances[msg.sender];
    require(balance >= amount, "insufficient");

    balances[msg.sender] = balance - amount;

    (bool ok, ) = msg.sender.call{value: amount}("");
    require(ok, "transfer failed");
}
```

The broader invariant is:

```text
effects on internal state
        ↓
external interaction
```

rather than the reverse.

## 40.3 Integer/accounting invariants

The important question is not only:

> “Can arithmetic overflow?”

but:

> “Can the contract's accounting state diverge from the actual asset state?”

Examples:

```text
total shares = sum(user shares)
assets backing shares >= liabilities
withdrawal <= claimable amount
fee <= configured maximum
```

## 40.4 Oracle manipulation

If price-sensitive state depends on a weak spot price:

```text
attacker influences market state
        |
        v
oracle reads manipulated price
        |
        v
contract computes incorrect collateral/value
        |
        v
attacker extracts value
```

Secure designs commonly use robust oracle architectures, time-weighted observations where appropriate, sanity bounds, liquidity assumptions, and circuit breakers.

---

# 41. Oracles, Pricing, and Economic Invariants

A DeFi protocol is a combination of code + economics.

Example lending invariant:

```text
borrowed_value <= collateral_value × liquidation_threshold
```

If an oracle gives an attacker an artificially high collateral value, the software can remain logically correct while the *economic invariant* fails.

Audit questions:

1. Where does the price come from?
2. How fresh is it?
3. What happens during low liquidity?
4. Can a single market influence it?
5. Is there a fallback?
6. Can the same asset be recursively rehypothecated?
7. Does a flash loan change assumptions?
8. What happens if a dependency is paused or malicious?

The best smart-contract auditors think in **invariants**, not just individual functions.

---

# 42. Bridges and Cross-Chain Systems

A simplified bridge:

```text
Chain A                                   Chain B
--------                                  --------
User                                      User
  |                                         ^
  v                                         |
Bridge A ── message/proof ──> Relayer ──> Bridge B
  |                                         |
  v                                         v
Lock/Burn                               Mint/Release
```

The critical security property is:

```text
asset released on B
   only if
valid state transition on A was proven
```

Typical trust assumptions include:

- validator sets;
- multisignature thresholds;
- light-client proofs;
- optimistic challenge mechanisms;
- message uniqueness;
- nonce ordering.

A cross-chain system can fail if:

```text
message authentication is bypassed
OR
message replay is possible
OR
validator threshold is insufficient
OR
proof verification is incorrect
OR
asset accounting is inconsistent
```

---

# 43. Governance and Privileged Roles

Governance security is often overlooked.

Architecture:

```text
Users
  |
  v
Governance Proposal
  |
  v
Voting / Timelock
  |
  v
Privileged Contract Action
  |
  v
Protocol State
```

Important controls:

- quorum;
- voting power calculation;
- delegation;
- proposal validation;
- timelocks;
- emergency roles;
- cancellation mechanisms;
- upgrade authorization.

A protocol can be technically “non-reentrant” and still be catastrophically unsafe if an unauthorized actor can become a privileged governor.

---

# 44. Browser and Mobile Security Boundaries

Browsers enforce multiple security boundaries:

```text
Web origin
  |
  +--> DOM
  +--> cookies
  +--> storage
  +--> network permissions
  +--> frames
  +--> workers
```

A browser vulnerability becomes severe when code crosses these boundaries without intended authority.

Mobile applications add:

```text
App sandbox
  |
  +--> app-private storage
  +--> permissions
  +--> IPC
  +--> WebView
  +--> OS services
```

A strong security mental model is:

```text
attacker input
   ↓
application parser
   ↓
memory/object model
   ↓
sandbox boundary
   ↓
OS privilege boundary
```

The higher the boundary crossed, the greater the potential impact.

---

# 45. Sandbox Escapes and Kernel Security: Conceptual Model

This section deliberately stays conceptual because advanced kernel exploitation can become operationally dangerous.

A sandbox escape generally looks like:

```text
untrusted content
      |
      v
memory/object/parser bug
      |
      v
code execution inside sandbox
      |
      v
sandbox boundary crossed
      |
      v
higher-privileged process / service
      |
      v
OS-level capabilities
```

A kernel vulnerability adds another boundary:

```text
user-space process
      |
      | syscall / IPC / driver interface
      v
kernel
      |
      v
system-wide privilege
```

Security engineers should learn:

- memory safety concepts;
- process isolation;
- capability boundaries;
- syscall surfaces;
- IPC models;
- privilege separation;
- mitigations such as ASLR, DEP/NX, sandboxing, CFI, hardened allocators.

For defensive work, the core question remains:

> What untrusted data reaches a privileged parser or privileged service, and what assumptions does it make about that data?

---

# 46. Exploit Chains: From Primitive to Critical Impact

High-severity vulnerabilities frequently form chains.

A general chain:

```text
Primitive
   |
   v
Initial execution / data access
   |
   v
Credential / privilege discovery
   |
   v
Privilege escalation
   |
   v
Sensitive resource access
   |
   v
Persistence / lateral movement
```

Example conceptual chain:

```text
SSRF
  ↓
internal credential exposure
  ↓
cloud API access
  ↓
privileged storage read
  ↓
sensitive data
```

Another:

```text
IDOR
  ↓
admin-only object modification
  ↓
privileged account state change
  ↓
account takeover
```

Another:

```text
stored XSS
  ↓
privileged operator views content
  ↓
privileged UI action
  ↓
administrative impact
```

The researcher should document each link separately and show that the chain is causal, not speculative.

---

# 47. Secure Architecture Patterns

## 47.1 Defense in depth

```text
             +------------------------+
             | Input validation       |
             +------------------------+
                       |
             +------------------------+
             | Authentication         |
             +------------------------+
                       |
             +------------------------+
             | Authorization          |
             +------------------------+
                       |
             +------------------------+
             | Least privilege        |
             +------------------------+
                       |
             +------------------------+
             | Network isolation      |
             +------------------------+
                       |
             +------------------------+
             | Monitoring / detection |
             +------------------------+
```

A missing layer should not immediately become catastrophic.

## 47.2 Zero trust between services

Do not assume:

```text
internal network = trusted
```

Prefer:

```text
service identity
+ authenticated channel
+ explicit authorization
+ least privilege
```

## 47.3 Secure-by-construction APIs

Instead of:

```python
update_object(request.json)
```

use explicit commands:

```python
update_profile(display_name, timezone)
change_password(current_password, new_password)
transfer_money(source, destination, amount)
```

Narrow operations are easier to secure than generic mutation primitives.

---

# 48. Secure Coding Patterns

## Input validation

Validation is useful when it expresses business policy.

```python
if not isinstance(amount, int):
    raise BadRequest()
if amount <= 0:
    raise BadRequest()
if amount > MAX_TRANSFER:
    raise BadRequest()
```

## Output encoding

Encode according to the interpreter context.

## Parameterization

Use prepared statements for SQL values.

## Allowlists

For security-sensitive choices, allowlist valid values.

## Least privilege

Run processes with the minimum required permissions.

## Fail closed

If authorization information is missing or invalid:

```text
deny
```

rather than:

```text
assume allow
```

## Explicit state machines

Represent security-sensitive transitions:

```text
DRAFT
  ↓ submit
PENDING_REVIEW
  ↓ approve
APPROVED
  ↓ execute
COMPLETED
```

Do not allow arbitrary field manipulation to skip states.

---

# 49. Testing Strategy

Secure code needs more than unit tests.

## Test pyramid

```text
                 /\
                /  \
               / E2E\
              /------\
             /Integration\
            /------------\
           / Unit tests  \
          /--------------\
```

Add security-specific layers:

```text
Unit security invariants
Integration authorization tests
Property-based tests
Fuzzing
Static analysis
Dependency scanning
Dynamic application testing
Manual threat-model review
```

## Authorization matrix

Create explicit tests:

| Caller | Resource owner | Expected |
|---|---|---|
| user A | user A | allow |
| user A | user B | deny |
| org admin | org member | allow according to policy |
| tenant A | tenant B | deny |
| anonymous | public | allow if public |
| anonymous | private | deny |

This table catches many access-control failures immediately.

---

# 50. How to Read a Codebase Like a Security Researcher

Do not read files from top to bottom first.

Use a security-oriented map.

## Step 1: Find entry points

Search for:

```text
routes
controllers
handlers
resolvers
webhooks
RPC methods
CLI entry points
message consumers
```

## Step 2: Mark sources

Find user-controlled inputs:

```text
query params
path params
JSON body
cookies
headers
uploads
webhook bodies
queue messages
external API responses
```

## Step 3: Mark sinks

Find dangerous or privileged operations:

```text
SQL execution
shell execution
file access
network requests
HTML rendering
admin mutations
credential issuance
money movement
contract calls
cloud API calls
```

## Step 4: Trace data flow

Use:

```text
source → transformation → validation → authorization → sink
```

## Step 5: Identify missing checks

The most interesting lines are often where you expected to see a security control and do not.

---

# 51. How to Build a Mental Model Efficiently

The goal is not memorizing payloads. It is recognizing invariant violations.

## Level 1: Know the boundary

Examples:

```text
browser ↔ server
user ↔ admin
tenant A ↔ tenant B
application ↔ database
application ↔ internal network
application ↔ cloud IAM
contract ↔ external contract
```

## Level 2: Know the authority

Ask:

```text
Who has permission?
Who supplies the identity?
Who verifies it?
Where is the permission checked?
```

## Level 3: Know the interpreter

Ask:

```text
What parser interprets this data?
SQL?
Shell?
HTML?
JSON parser?
Template?
Solidity ABI decoder?
Cloud API?
```

## Level 4: Know the state machine

Ask:

```text
What states exist?
What transitions are allowed?
Can a request skip a state?
Can a transition happen twice?
Can it happen concurrently?
```

## Level 5: Know the blast radius

Ask:

```text
one object?
one user?
one tenant?
all tenants?
control plane?
financial assets?
```

## The 7-question habit

Whenever you inspect an endpoint, ask:

```text
1. Who can call it?
2. What can they control?
3. What identity does the server use?
4. What resource does it touch?
5. What privileged operation occurs?
6. What assumption connects input to privilege?
7. What happens if the assumption is false?
```

This habit generalizes across nearly every vulnerability class.

---

# 52. A Practical Local Lab

Build a deliberately vulnerable application locally.

Recommended components:

```text
                Browser / API client
                         |
                         v
                    +---------+
                    | FastAPI | 
                    +----+----+
                         |
              +----------+----------+
              |          |          |
              v          v          v
           SQLite      Redis      File store
```

Create endpoints for:

```text
/login
/users/{id}
/profile
/password/reset
/oauth/callback
/upload
/proxy
/orders
/coupon
/transfer
/webhook
```

Implement each once incorrectly and once correctly.

## Local exercise structure

For every vulnerability:

```text
1. Write vulnerable code.
2. Write a unit test that demonstrates the broken invariant.
3. Fix the code.
4. Re-run the same test.
5. Add a regression test.
6. Write the security invariant in plain English.
```

Example:

```python
def test_user_cannot_read_another_users_invoice(client, user_a, user_b):
    login_as(client, user_a)
    response = client.get(f"/api/invoices/{user_b.invoice_id}")
    assert response.status_code in (403, 404)
```

That exercise teaches more than memorizing a payload.

---

# 53. Bug Bounty Reporting and Severity Reasoning

A strong report should answer:

```text
What is the vulnerability?
Where is it?
Who can exploit it?
What exact security boundary is crossed?
What is the demonstrated impact?
How is it reproduced safely?
How can it be fixed?
```

A good structure:

```text
Title
Summary
Affected component
Security impact
Root cause
Reproduction in authorized environment
Evidence
Attack chain, if applicable
Remediation
Regression test recommendation
```

## Severity reasoning

Consider:

```text
Privileges required
User interaction required
Remote/local
Confidentiality impact
Integrity impact
Availability impact
Scope
Scale
Exploit reliability
Persistence
Financial impact
```

Do not exaggerate impact. A credible report with carefully demonstrated impact is much more persuasive than a speculative “could lead to RCE.”

---

# 54. Common False Positives and Dead Ends

## “There is an ID parameter, therefore IDOR.”

Wrong.

The server may perform strict ownership checks.

## “There is SSRF because the server accepts a URL.”

Not necessarily.

The destination may be restricted to a hardened allowlist and outbound proxy.

## “There is XSS because special characters appear in HTML.”

Only if the browser interprets the output as executable content in a security-relevant context.

## “JWT contains admin=true.”

That claim may be rejected by signature validation or ignored by authorization logic.

## “Endpoint lacks a UI control.”

UI restrictions are not equivalent to server authorization, but the absence of a UI button alone proves nothing.

## “I got a 500 error.”

An error is not automatically a vulnerability. Determine whether a security boundary was crossed or sensitive behavior exposed.

---

# 55. Security Review Checklist

## Identity

```text
[ ] Authentication is explicit
[ ] Sessions rotate on privilege transitions
[ ] Reset tokens are random and one-time
[ ] MFA completion is required before full privilege
[ ] Recovery paths are equally strong
```

## Authorization

```text
[ ] Object-level access checks exist
[ ] Tenant isolation is enforced server-side
[ ] Property-level authorization is enforced
[ ] Admin actions use explicit authorization
[ ] Authorization does not depend on hidden UI controls
```

## Input

```text
[ ] SQL uses parameterization
[ ] Shell execution avoided or safely parameterized
[ ] URLs use destination policies
[ ] Uploads are validated and isolated
[ ] Paths are canonicalized and contained
[ ] JSON schemas are strict
```

## Browser

```text
[ ] Context-aware output encoding
[ ] CSRF protection for cookie-authenticated mutations
[ ] CORS allowlist is explicit
[ ] Security headers are appropriate
[ ] Sensitive responses are not inadvertently cacheable
```

## Cloud

```text
[ ] Workload IAM is least privilege
[ ] Egress is restricted
[ ] Metadata access is protected
[ ] Secrets are not embedded in code
[ ] Object storage is private by default
[ ] Signed links are scoped and short-lived
```

## Distributed systems

```text
[ ] Internal services authenticate each other
[ ] Messages have schemas and provenance
[ ] Webhooks verify signatures
[ ] Replay protection exists
[ ] Idempotency is handled
[ ] Race conditions have explicit tests
```

## Smart contracts

```text
[ ] Privileged functions have correct access control
[ ] Reentrancy is considered
[ ] Accounting invariants are explicit
[ ] Oracle assumptions are tested
[ ] External calls are bounded and checked
[ ] Upgrade/admin paths are reviewed
[ ] Cross-contract trust assumptions are documented
```

---

# 56. Further Learning Roadmap

A high-performance learning order is:

## Phase 1 — HTTP and application fundamentals

Learn deeply:

- HTTP request/response semantics;
- cookies;
- browser origins;
- sessions;
- REST APIs;
- JSON;
- databases;
- SQL;
- reverse proxies;
- caching.

Goal:

```text
Browser → proxy → application → database
```

You should be able to explain every trust boundary.

## Phase 2 — Authentication and authorization

Master:

- sessions;
- cookies;
- OAuth/OIDC;
- MFA;
- password recovery;
- RBAC/ABAC;
- IDOR/BOLA;
- tenant isolation.

This is the highest-value conceptual foundation for web security.

## Phase 3 — Injection and server-side primitives

Master:

- SQL injection;
- command injection;
- template injection;
- path traversal;
- file upload;
- SSRF;
- deserialization.

Learn each using the same mental model:

```text
source → parser/interpreter → sink → authority → impact
```

## Phase 4 — Business logic

Learn to model:

```text
state
transitions
invariants
concurrency
money
ownership
```

This is where many mature applications still fail.

## Phase 5 — Cloud and distributed systems

Learn:

- IAM;
- queues;
- object storage;
- service identities;
- metadata services;
- Kubernetes concepts;
- service-to-service authorization;
- secrets.

## Phase 6 — Smart contracts

Learn:

- Solidity;
- EVM execution;
- storage;
- calls/delegatecall;
- ERC standards;
- access control;
- reentrancy;
- accounting;
- oracle design;
- DeFi mechanics;
- bridges;
- governance.

## Phase 7 — Advanced systems

Then move into:

- browser internals;
- memory safety;
- operating systems;
- sandboxing;
- kernel architecture;
- exploit mitigations;
- compiler behavior.

At that point, the same mental model still applies:

```text
untrusted input
      ↓
parser / state machine
      ↓
security boundary
      ↓
privileged capability
```

---

# The Unifying Mental Model

All of these topics look different on the surface:

```text
IDOR
SQLi
SSRF
XSS
CSRF
RCE
OAuth bugs
Cloud IAM flaws
DeFi exploits
Bridge failures
Kernel bugs
```

But underneath, they repeatedly reduce to a small number of concepts.

## Concept 1: Confused authority

```text
lower privilege
      ↓
input
      ↓
higher privilege action
```

Examples:

- user modifies admin field;
- tenant A reads tenant B;
- webhook forges privileged event;
- service identity performs user-selected action.

## Concept 2: Confused data and code

```text
data → interpreter → instructions
```

Examples:

- SQL injection;
- command injection;
- XSS;
- template injection.

## Concept 3: Confused destination

```text
attacker-controlled name
        ↓
trusted network/filesystem/resource
```

Examples:

- SSRF;
- path traversal;
- open redirects;
- unsafe object storage keys.

## Concept 4: Confused identity

```text
attacker-controlled or weakly verified identity
             ↓
trusted session / token / principal
```

Examples:

- session fixation;
- password reset bugs;
- OAuth confusion;
- JWT validation failures.

## Concept 5: Confused state

```text
state A
  ↓
state transition
  ↓
state B
```

If the system permits an illegal transition, business logic fails.

Examples:

- double spending;
- replayed coupon;
- repeated withdrawal;
- unauthorized approval.

## Concept 6: Confused trust across systems

```text
System A says “trusted”
       ↓
System B accepts without independent verification
```

Examples:

- internal microservice trust;
- webhook spoofing;
- bridge message validation;
- cloud role abuse.

---

# A Final Security Researcher Worksheet

For any feature, write this down before testing it:

```text
FEATURE:

ENTRY POINT:

CALLER:

AUTHENTICATION:

AUTHORIZATION:

TENANT / OWNERSHIP BOUNDARY:

ATTACKER-CONTROLLED INPUTS:

PARSER / INTERPRETER:

SECURITY-SENSITIVE SINK:

TRUSTED IDENTITY USED BY SERVER:

STATE TRANSITIONS:

SECURITY INVARIANTS:

RATE / REPLAY CONTROLS:

EXTERNAL DEPENDENCIES:

PRIVILEGE AVAILABLE TO PROCESS:

POTENTIAL IMPACT:

POSSIBLE CHAIN:

SAFE LOCAL REPRODUCTION:

REMEDIATION:
```

Then ask the most important question:

> **What must be true for this operation to be safe, and where is that truth actually enforced?**

That question is the bridge from “knowing vulnerability names” to actually thinking like a security engineer.

---

# Appendix A — Vulnerable vs Secure Pattern Summary

| Topic | Vulnerable pattern | Secure pattern |
|---|---|---|
| Authentication | plaintext/fast password comparison | password hashing + throttling |
| Sessions | predictable/reused IDs | random IDs + rotation + secure cookies |
| Authorization | object lookup by ID only | object + principal/tenant authorization |
| SQL | string concatenation | parameterized queries |
| Shell | string command | argument array / dedicated API |
| SSRF | unrestricted URL fetch | allowlist + egress enforcement |
| Upload | save to web root | private/object storage + content validation |
| Path access | string concatenation | canonicalize + containment check |
| XSS | raw HTML insertion | contextual encoding / safe DOM APIs |
| CSRF | cookie-authenticated mutation without protection | SameSite + CSRF token/origin controls |
| CORS | reflect arbitrary origin | explicit origin allowlist |
| OAuth | arbitrary redirect URL | registered exact redirects + state + PKCE |
| JWT | trust decoded payload | verify signature + issuer/audience/algorithm |
| Webhooks | trust body/IP | raw-body signature + replay protection |
| Cloud | broad service role | least-privilege workload identity |
| Microservices | network trust | service identity + explicit authz |
| Queue | blind event trust | schema + provenance + idempotency |
| Smart contract | unrestricted privileged call | strict access control + invariants |
| DeFi oracle | single manipulable spot source | robust oracle + sanity limits |

---

# Appendix B — How to Turn This Into Skill

For each topic, do four exercises:

```text
READ
  ↓
BUILD
  ↓
BREAK IN LOCAL LAB
  ↓
FIX + WRITE REGRESSION TEST
```

Example for IDOR:

```text
Read authorization model
        ↓
Build /users/{id}
        ↓
Create two users locally
        ↓
Prove cross-user access fails after fix
        ↓
Write authorization matrix tests
```

For SSRF:

```text
Build local target service
        ↓
Build fetch endpoint
        ↓
Demonstrate unintended internal reachability
        ↓
Add destination policy + egress restriction
        ↓
Regression test blocked destinations
```

For SQL injection:

```text
Build intentionally vulnerable query
        ↓
Observe query/data boundary failure locally
        ↓
Convert to parameterized query
        ↓
Add tests for malicious-looking data
```

For smart contracts:

```text
Write state invariant
        ↓
Implement naive contract
        ↓
Write invariant test
        ↓
Add access/reentrancy/accounting protection
        ↓
Run invariant/property tests
```

The point is not to become a person who remembers the most payloads.

The point is to become someone who can look at a system and immediately see:

```text
INPUT
  ↓
ASSUMPTION
  ↓
TRUST BOUNDARY
  ↓
AUTHORITY
  ↓
STATE CHANGE
  ↓
IMPACT
```

Once that becomes automatic, new vulnerability classes become much easier to learn because you are learning **security concepts**, not isolated tricks.

---

# Appendix C — Safe References for Continued Study

Prefer primary or high-quality defensive references while studying:

- OWASP Application Security Verification Standard (ASVS)
- OWASP Web Security Testing Guide (WSTG)
- OWASP API Security Top 10
- OWASP Cheat Sheet Series
- PortSwigger Web Security Academy
- NIST secure software development guidance
- CWE and MITRE entries for vulnerability classes
- RFC specifications for HTTP, OAuth 2.0, OAuth PKCE, JWT/JWS/JWE, and related protocols
- Official cloud-provider security documentation
- Official Solidity/EVM documentation and reputable smart-contract auditing resources

Always check the specific bug-bounty program's current rules before testing a live target. Scope, prohibited techniques, rate limits, safe-harbor terms, and reward criteria can change.

---

## Closing Principle

The strongest security mindset is not:

> “I know how to exploit XSS.”

It is:

> “I know what the application believes, why it believes it, where that belief is enforced, and what happens if the belief is false.”

That is the mental model that transfers across web applications, APIs, cloud systems, mobile software, browsers, operating systems, and smart contracts.

---

# 57. Comprehensive Test-Case Catalog

This catalog turns the concepts into repeatable security tests. Each case is deliberately phrased as a **security property** rather than a payload. For live bug bounty work, adapt the case to the program's exact scope, rate limits, and safe-harbor rules.

## 57.1 Test Case Anatomy

Use this structure for every test:

```text
TEST ID
  |
  +--> Asset / endpoint
  |
  +--> Actor / role
  |
  +--> Preconditions
  |
  +--> Baseline request
  |
  +--> Single controlled variation
  |
  +--> Expected invariant
  |
  +--> Observed result
  |
  +--> Security impact
  |
  +--> Evidence
  |
  +--> Regression test
```

### Evidence hierarchy

Prefer evidence in this order:

```text
Observed security-boundary crossing
        >
Reliable authorization bypass
        >
Repeatable state corruption
        >
Direct sensitive-data access
        >
Strong root-cause proof
        >
Speculative downstream impact
```

Do not turn speculation into an assertion of impact.

---

## 57.2 Authentication Test Matrix

| ID | Property | Test | Secure result |
|---|---|---|---|
| AUTH-001 | Account enumeration | Compare unknown vs known identity | Responses do not unnecessarily reveal existence |
| AUTH-002 | Password policy | Boundary and policy-invalid passwords | Rejected according to policy |
| AUTH-003 | Login throttling | Controlled repeated failures in a lab | Rate limit / progressive defense applies |
| AUTH-004 | Credential stuffing resistance | Test synthetic credential set | Defenses limit automated abuse |
| AUTH-005 | Session creation | Compare anonymous and authenticated requests | Server establishes explicit authenticated state |
| AUTH-006 | Authentication transition | Login from an existing anonymous session | Session identifier rotates |
| AUTH-007 | Logout | Reuse prior session after logout | Rejected according to policy |
| AUTH-008 | Password change | Use old password after successful change | Old credential is invalidated |
| AUTH-009 | Password reset | Reuse reset token | Token is single-use |
| AUTH-010 | Password reset expiry | Use expired token | Rejected |
| AUTH-011 | Reset account binding | Attempt token against a different test identity | Rejected |
| AUTH-012 | Recovery flow | Compare strength of recovery vs login | Recovery does not bypass core identity proof |
| AUTH-013 | MFA state | Call protected action before MFA completion | Denied |
| AUTH-014 | MFA replay | Reuse completed challenge | Rejected |
| AUTH-015 | MFA session binding | Use challenge/session across identities | Rejected |

### Test reasoning

The critical mental model is:

```text
Credential proof
      |
      v
Identity established
      |
      v
Session created
      |
      v
Privilege level established
      |
      v
Sensitive operation
```

If the application permits the last step without every required earlier state transition, the chain is broken.

---

## 57.3 Authorization / BOLA / IDOR Matrix

For each protected resource, create at least these synthetic identities:

```text
user_A
user_B
manager_A
admin_A
user_C_tenant_2
suspended_user
expired_session_user
```

Then test the Cartesian product of role, object, action, and state.

| ID | Test | Expected |
|---|---|---|
| AUTHZ-001 | A reads A object | Allowed |
| AUTHZ-002 | A reads B object | Denied |
| AUTHZ-003 | A modifies B object | Denied |
| AUTHZ-004 | B modifies A object | Denied |
| AUTHZ-005 | A accesses tenant-2 object | Denied |
| AUTHZ-006 | A changes role field of own account | Only if explicitly authorized |
| AUTHZ-007 | User calls admin endpoint | Denied |
| AUTHZ-008 | Manager accesses unrelated tenant | Denied |
| AUTHZ-009 | User accesses deleted object | Lifecycle policy enforced |
| AUTHZ-010 | User accesses pending privileged object | State + ownership enforced |
| AUTHZ-011 | Nested object bypass | Parent and child checks both enforced |
| AUTHZ-012 | Alternate endpoint | Same authorization policy applies |
| AUTHZ-013 | API/UI discrepancy | Server remains authoritative |
| AUTHZ-014 | Bulk endpoint | Every returned object is authorized |
| AUTHZ-015 | Export/report endpoint | Dataset is filtered by caller authority |

### Secure design pattern

```python
@dataclass(frozen=True)
class Principal:
    user_id: str
    tenant_id: str
    roles: frozenset[str]


def get_document(db, principal: Principal, document_id: str):
    row = db.fetch_one(
        """
        SELECT id, owner_id, tenant_id, title, body
        FROM documents
        WHERE id = %s
          AND tenant_id = %s
        """,
        (document_id, principal.tenant_id),
    )

    if row is None:
        raise NotFound()

    if row["owner_id"] != principal.user_id and "document:read:any" not in principal.roles:
        raise Forbidden()

    return row
```

This is preferable to fetching a record globally and checking only one field later because the tenant boundary is pushed closer to data retrieval.

---

## 57.4 Account-Takeover Chain Tests

Account takeover is best tested as a **chain of security properties**.

```text
Can attacker influence identity selection?
            |
            v
Can attacker obtain or forge proof?
            |
            v
Can attacker attach proof to victim?
            |
            v
Can attacker create authenticated state?
            |
            v
Can attacker perform sensitive actions?
```

Test families:

| ID | Area | Security question |
|---|---|---|
| ATO-001 | Password reset | Is reset token bound to intended account? |
| ATO-002 | Email change | Does changing email require appropriate proof? |
| ATO-003 | Phone change | Does phone recovery preserve account ownership? |
| ATO-004 | OAuth linking | Must both identities be proven before linking? |
| ATO-005 | Session reuse | Can a retired session continue? |
| ATO-006 | MFA recovery | Is recovery stronger than the protected action? |
| ATO-007 | Invitation flow | Can invitations attach a privileged role to the wrong identity? |
| ATO-008 | Magic link | Is token scope and lifetime correct? |
| ATO-009 | Support impersonation | Is internal impersonation narrowly controlled and audited? |
| ATO-010 | Device trust | Can device enrollment be transferred or replayed? |

---

## 57.5 OAuth / OIDC Test Matrix

| ID | Test | Secure expectation |
|---|---|---|
| OAUTH-001 | Missing/invalid state | Authorization response rejected |
| OAUTH-002 | State reuse | Response cannot be replayed |
| OAUTH-003 | Nonce mismatch | ID token rejected |
| OAUTH-004 | Wrong issuer | Token rejected |
| OAUTH-005 | Wrong audience | Token rejected |
| OAUTH-006 | Invalid signature | Token rejected |
| OAUTH-007 | Expired token | Rejected |
| OAUTH-008 | Redirect URI mismatch | Authorization server/client policy blocks it |
| OAUTH-009 | Account linking | External subject mapped to correct local identity |
| OAUTH-010 | Token substitution | Token for one client/issuer not accepted for another |
| OAUTH-011 | Scope escalation | Client cannot silently obtain unauthorized scopes |
| OAUTH-012 | PKCE | Appropriate public-client flows require verifier binding |

### Identity mapping principle

Never treat these as interchangeable:

```text
email
username
provider account name
oauth subject
local user ID
organization ID
session ID
```

Stable provider identifiers such as an issuer + subject pair are much stronger identity keys than mutable presentation fields such as display names or email addresses when designing OIDC mappings.

---

## 57.6 Business-Logic Matrix

| ID | Scenario | Invariant |
|---|---|---|
| BIZ-001 | Apply discount | Discount cannot exceed allowed policy |
| BIZ-002 | Reuse coupon | One-time coupon cannot be reused |
| BIZ-003 | Refund twice | One transaction cannot be refunded twice |
| BIZ-004 | Cancel after shipment | Illegal state transition is rejected |
| BIZ-005 | Approve own request | Separation-of-duties rule remains enforced |
| BIZ-006 | Change price after quote | Quote integrity is preserved |
| BIZ-007 | Negative amount | Monetary quantity domain is enforced |
| BIZ-008 | Zero amount | Zero-value operation cannot bypass required conditions |
| BIZ-009 | Quantity overflow | Maximum is enforced in domain and storage |
| BIZ-010 | Invite reuse | Invitation state is one-time where required |
| BIZ-011 | Trial reuse | Eligibility is recomputed server-side |
| BIZ-012 | Subscription downgrade | Proration and entitlement rules remain consistent |
| BIZ-013 | Transfer asset | Sender and receiver constraints are atomic |
| BIZ-014 | State skipping | Required workflow steps cannot be skipped |
| BIZ-015 | Replay | Same business command does not create unintended duplicate effects |

### Business-logic review technique

Draw the valid state graph first:

```text
             +------------+
             |            |
             v            |
DRAFT --> SUBMITTED --> APPROVED --> EXECUTED
  |            |             |          |
  v            v             v          v
CANCELLED   REJECTED      REJECTED    REFUNDED
```

Then enumerate all externally callable methods that can move each state.

---

## 57.7 SQL / Injection Test Matrix

Do not begin with a payload dictionary. Begin by finding interpreter boundaries.

| ID | Test | Secure expectation |
|---|---|---|
| INJ-001 | String boundary | Data does not modify SQL structure |
| INJ-002 | Numeric type | Invalid type rejected or safely normalized |
| INJ-003 | Sort field | Field selected from allowlist |
| INJ-004 | Filter field | Field selected from allowlist |
| INJ-005 | Search field | Parameterized query |
| INJ-006 | Bulk query | IDs passed as typed parameters |
| INJ-007 | Stored query fragments | Data cannot inject a new clause |
| INJ-008 | Error path | Database implementation details not exposed |

### Safe dynamic SQL pattern

Parameters cannot usually substitute identifiers such as column names. Use an allowlist:

```python
SORT_COLUMNS = {
    "newest": "created_at DESC",
    "oldest": "created_at ASC",
    "price": "price ASC",
}

sort = SORT_COLUMNS.get(request.args.get("sort"), "created_at DESC")

query = f"SELECT id, price FROM products ORDER BY {sort} LIMIT %s"
rows = db.execute(query, (100,))
```

The critical control is that `sort` comes from a finite server-side set rather than raw user input.

---

## 57.8 XSS Test Matrix

| ID | Context | Secure expectation |
|---|---|---|
| XSS-001 | HTML text | Context-aware escaping |
| XSS-002 | HTML attribute | Attribute encoding / safe framework binding |
| XSS-003 | URL | Scheme + destination policy and encoding |
| XSS-004 | JS string | Avoid interpolation; use safe data serialization |
| XSS-005 | DOM sink | Untrusted data never reaches executable sink |
| XSS-006 | Stored content | Rendered content cannot execute in another user's security context |
| XSS-007 | Admin view | High-privilege users are not exposed to unsafe rendering |
| XSS-008 | CSP | Defense-in-depth policy exists where appropriate |

Safe DOM example:

```javascript
const title = document.querySelector("#title");
title.textContent = profile.displayName;
```

Risky pattern:

```javascript
title.innerHTML = profile.displayName;
```

The distinction is not "one function is always bad." It is whether untrusted data is inserted into an execution-capable context.

---

## 57.9 SSRF Test Matrix

| ID | Test | Secure expectation |
|---|---|---|
| SSRF-001 | Scheme allowlist | Only intended schemes accepted |
| SSRF-002 | Host allowlist | Only approved destinations accepted |
| SSRF-003 | Redirect | Every redirect destination is revalidated |
| SSRF-004 | DNS resolution | Private/reserved destinations blocked |
| SSRF-005 | DNS rebinding class | Validation cannot be bypassed by address changes |
| SSRF-006 | Egress policy | Application network cannot freely reach sensitive networks |
| SSRF-007 | Response size | Large response is bounded |
| SSRF-008 | Timeout | Slow destinations cannot exhaust workers |
| SSRF-009 | Protocol confusion | Unsupported protocols rejected |
| SSRF-010 | Credential headers | User credentials are not automatically forwarded |

---

## 57.10 File / Path Test Matrix

| ID | Test | Secure expectation |
|---|---|---|
| FILE-001 | Filename | Server-generated name |
| FILE-002 | Path boundary | Canonicalized path remains under intended root |
| FILE-003 | File type | Server validates type/content |
| FILE-004 | Extension mismatch | Content does not determine executable behavior accidentally |
| FILE-005 | Size | Hard upper bound |
| FILE-006 | Archive extraction | Extraction cannot escape destination directory |
| FILE-007 | Download authorization | Object-level permission checked |
| FILE-008 | Temporary files | Permissions prevent unintended access |
| FILE-009 | Conversion service | Runs with least privilege |
| FILE-010 | Stored files | Upload location is not executable |

---

## 57.11 Concurrency Test Matrix

| ID | Resource | Invariant |
|---|---|---|
| RACE-001 | Wallet/balance | Balance never goes below policy limit |
| RACE-002 | Coupon | One redemption per policy |
| RACE-003 | Inventory | Stock cannot become negative |
| RACE-004 | Approval | One approval decision per workflow |
| RACE-005 | Password reset | Token cannot be used concurrently twice |
| RACE-006 | Membership | Limits cannot be bypassed through simultaneous requests |
| RACE-007 | File creation | Unique-name invariant holds |
| RACE-008 | Idempotency | One command key creates one effect |

### Race review questions

```text
Is there a check-before-write?
Is there a write-before-external-side-effect?
Is the resource locked or conditionally updated?
Is there a unique constraint?
Can the same message be delivered twice?
Can a worker retry after a timeout?
```

---

## 57.12 API Mass-Assignment Test Matrix

A classic bug occurs when request fields map directly to model fields.

Vulnerable conceptual pattern:

```python
user.update(**request.json)
```

Secure pattern:

```python
ALLOWED = {"display_name", "timezone"}

body = request.json
updates = {
    key: value
    for key, value in body.items()
    if key in ALLOWED
}

user.update(**updates)
```

Tests:

```text
MA-001  Add privileged role field -> ignored/rejected
MA-002  Add account ownership field -> ignored/rejected
MA-003  Add verification status -> ignored/rejected
MA-004  Add billing tier -> ignored/rejected
MA-005  Add internal flags -> ignored/rejected
```

---

## 57.13 GraphQL Test Matrix

```text
GQL-001  Resolver object-level authorization
GQL-002  Nested resolver authorization
GQL-003  Mutation authorization
GQL-004  Query depth/cost limits
GQL-005  Batch amplification
GQL-006  Sensitive field exposure
GQL-007  Introspection policy
GQL-008  Alias-based response amplification
GQL-009  Tenant isolation through nested objects
GQL-010  Mutation replay/idempotency
```

The key lesson is that **each resolver is a security boundary**. Top-level authentication does not imply object-level authorization.

---

## 57.14 Webhook Security Test Matrix

A webhook is an inbound message from another system.

```text
Partner
  |
  | signed event
  v
Webhook endpoint
  |
  +--> verify signature
  +--> verify timestamp/freshness
  +--> parse schema
  +--> deduplicate event ID
  +--> authorize event source
  v
Apply state transition
```

Tests:

```text
HOOK-001  Invalid signature -> rejected
HOOK-002  Wrong secret -> rejected
HOOK-003  Old signed event -> rejected or handled safely
HOOK-004  Duplicate event -> no duplicate side effect
HOOK-005  Unknown event type -> rejected/ignored safely
HOOK-006  Wrong tenant mapping -> rejected
HOOK-007  Malformed payload -> bounded failure
```

---

# 58. Architecture-Level Test Cases

Application bugs often come from distributed-system mismatches rather than one unsafe line of code.

## 58.1 Edge -> API -> Service -> DB

```text
Internet
   |
   v
[CDN/WAF]
   |
   v
[API Gateway]
   |
   +---- auth context
   |
   v
[Service]
   |
   +---- authorization
   |
   +---- validation
   |
   v
[Repository]
   |
   +---- tenant predicate
   |
   v
[Database]
```

Test each layer for **policy preservation**.

Example failure:

```text
Gateway verifies identity
        |
        v
Service trusts client-provided tenant_id
        |
        v
Repository queries by object_id only
        |
        v
Cross-tenant access
```

This is why security cannot be inferred from a single component in isolation.

---

## 58.2 Queue / Worker Architecture

```text
Client
  |
  v
API
  |
  v
Queue  --->  Dead Letter Queue
  |
  v
Worker
  |
  +--> Database
  +--> Storage
  +--> External API
```

Test:

```text
QUEUE-001  Can a low-privileged caller enqueue a privileged command?
QUEUE-002  Does the worker trust caller-supplied role fields?
QUEUE-003  Are jobs authenticated/signed where appropriate?
QUEUE-004  Are retries idempotent?
QUEUE-005  Can a stale job execute after permission revocation?
QUEUE-006  Are sensitive jobs isolated by queue/worker identity?
QUEUE-007  Can a poison message exhaust worker resources?
```

## 58.3 Microservice identity

Do not assume:

```text
internal network == trusted
```

Prefer:

```text
Service A
   |
   | authenticated request
   v
Service B
   |
   +--> validate service identity
   +--> validate caller context
   +--> authorize operation
```

A compromised service should not automatically become a platform administrator.

---

# 59. Security Regression Testing

Every confirmed bug should become a regression test.

## 59.1 Regression pattern

```text
Bug discovered
      |
      v
Write failing security test
      |
      v
Fix root cause
      |
      v
Test passes
      |
      v
Add test permanently to CI
```

## 59.2 Example: BOLA regression

```python
def test_cross_tenant_document_is_not_readable(client, tenant_a_user, tenant_b_document):
    client.login(tenant_a_user)

    response = client.get(
        f"/api/documents/{tenant_b_document.id}"
    )

    assert response.status_code in (403, 404)
```

## 59.3 Example: mass-assignment regression

```python
def test_role_cannot_be_mass_assigned(client, user):
    client.login(user)

    response = client.patch(
        "/api/profile",
        json={"display_name": "Alice", "role": "admin"},
    )

    assert response.ok
    assert client.get_current_user().role != "admin"
```

The test should assert the **security property**, not merely the absence of an old payload.

---

# 60. Tooling Field Guide

The right tool depends on the question.

## 60.1 Intercepting proxies

### Burp Suite

Best for:

```text
manual HTTP analysis
request/response comparison
repeater workflows
extensions
authorized web/API testing
```

### OWASP ZAP

Best for:

```text
open-source proxying
passive analysis
automation
API scanning
spidering
DAST workflows
```

Official: https://www.zaproxy.org/

## 60.2 Network / packet analysis

### Wireshark

Best for:

```text
protocol analysis
TLS handshake debugging
DNS behavior
service discovery in lab environments
unexpected network flows
```

Official: https://www.wireshark.org/

### tcpdump

Best for lightweight packet capture from terminal-based labs.

### mitmproxy

Best for programmable HTTP/HTTPS interception and Python-driven workflows.

Official: https://mitmproxy.org/

---

## 60.3 Discovery / HTTP enumeration

ProjectDiscovery's open-source ecosystem includes tools such as:

```text
subfinder  -> subdomain discovery
httpx      -> HTTP probing / metadata
naabu      -> port scanning
nuclei     -> template-based checks
```

Official: https://projectdiscovery.io/open-source

**Safety:** run discovery and scanning only on assets explicitly authorized by the owner/program.

---

## 60.4 Fuzzing

### ffuf

Useful for authorized web content discovery and parameterized fuzzing.

### feroxbuster

Useful for recursive discovery in controlled environments.

### wfuzz

Flexible web request fuzzing.

### AFL++

Coverage-guided native fuzzing.

Official: https://aflplus.plus/

### libFuzzer

In-process fuzzing, commonly integrated with sanitizers and compiler tooling.

### Hypothesis

Property-based testing for Python.

Official: https://hypothesis.readthedocs.io/

### Schemathesis

Property-based API testing generated from OpenAPI/GraphQL schemas.

Official: https://schemathesis.readthedocs.io/

---

## 60.5 SAST / code analysis

### Semgrep

Excellent for rule-based code patterns and custom security checks.

Official: https://semgrep.dev/

### CodeQL

Excellent for data-flow and repository-scale semantic analysis.

Official: https://codeql.github.com/

### Bandit

Python security linting.

### gosec

Go security analysis.

### SpotBugs + Find Security Bugs

Useful for Java security analysis.

---

## 60.6 Dependency / software supply chain

### OSV-Scanner

Open-source vulnerability scanning based on the OSV database.

Official: https://github.com/google/osv-scanner

### Trivy

Container, filesystem, repository, and dependency scanning.

Official: https://trivy.dev/

### Grype

Vulnerability scanner for container images and filesystems.

Official: https://github.com/anchore/grype

### Syft

SBOM generation.

Official: https://github.com/anchore/syft

### Renovate / Dependabot

Automated dependency-update workflows.

---

## 60.7 Mobile security

### Frida

Dynamic instrumentation.

Official: https://frida.re/

### MobSF

Mobile Security Framework for static and dynamic analysis workflows.

Official: https://mobsf.github.io/docs/

### jadx

Android DEX/APK decompiler.

Official: https://github.com/skylot/jadx

### apktool

APK resource and bytecode analysis.

Official: https://github.com/iBotPeaches/Apktool

### Android Studio / adb

Core Android development and device-debugging environment.

---

## 60.8 Reverse engineering

### Ghidra

Open-source reverse-engineering suite.

Official: https://ghidra-sre.org/

### radare2

Command-line framework for binary analysis.

Official: https://rada.re/

### Cutter

GUI reverse-engineering interface for radare2.

---

## 60.9 Web3 / smart contract

### Foundry

```text
forge   -> build/test
anvil   -> local EVM node
cast    -> chain interaction
chisel  -> Solidity REPL
```

Foundry documents fuzzing, invariant testing, symbolic testing, fork testing, and mutation testing.

Official: https://www.getfoundry.sh/

### Slither

Static analysis framework for Solidity.

Official: https://github.com/crytic/slither

### Echidna

Property-based smart-contract fuzzing framework.

Official: https://github.com/crytic/echidna

### Hardhat

JavaScript/TypeScript-oriented Ethereum development environment.

Official: https://hardhat.org/

---

# 61. Free and Open-Source Practice Platforms

## PortSwigger Web Security Academy

https://portswigger.net/web-security

Strongest for structured browser/API vulnerability practice. Labs are interactive and cover many modern web topics.

## OWASP Juice Shop

https://owasp.org/www-project-juice-shop/

Intentionally vulnerable modern application with many challenge types and different difficulty levels.

## OWASP WebGoat

https://owasp.org/www-project-webgoat/

Teaching application designed to demonstrate vulnerabilities and remediation.

## OWASP Vulnerable Web Applications Directory

https://vwad.owasp.org/

Directory of intentionally vulnerable applications for controlled training.

## DVWA

Damn Vulnerable Web Application for classic web-security practice.

https://github.com/digininja/DVWA

## crAPI

OWASP's vulnerable application focused heavily on API security.

https://github.com/OWASP/crAPI

## Metasploitable

Deliberately vulnerable environments for broader security training.

https://github.com/rapid7/metasploitable3

## Additional practice model

A very effective setup is to build your own miniature application:

```text
Frontend
   |
   v
API
   |
   +--> Auth
   +--> Orders
   +--> Files
   +--> Admin
   +--> Payments (fake)
   |
   v
PostgreSQL
```

Then create two versions:

```text
vulnerable branch
secure branch
```

Each vulnerability gets:

```text
bug
  -> failing test
  -> exploit reproduction in local lab
  -> code fix
  -> regression test
  -> security review note
```

This is one of the fastest ways to convert passive knowledge into engineering skill.

---

# 62. Building a Personal Security Test Harness

A high-quality test harness makes security reasoning repeatable.

## Suggested project layout

```text
security-lab/
├── apps/
│   ├── vulnerable-api/
│   └── secure-api/
├── infra/
│   ├── docker-compose.yml
│   └── seed.sql
├── tests/
│   ├── auth/
│   ├── authorization/
│   ├── business_logic/
│   ├── injection/
│   ├── concurrency/
│   └── graphql/
├── tooling/
│   ├── scripts/
│   └── schemas/
├── reports/
│   └── examples/
└── README.md
```

## Minimal Docker Compose architecture

```yaml
services:
  app:
    build: ./apps/secure-api
    ports:
      - "127.0.0.1:8000:8000"
    environment:
      DATABASE_URL: postgresql://lab:lab@db:5432/lab

  db:
    image: postgres:16
    environment:
      POSTGRES_USER: lab
      POSTGRES_PASSWORD: lab
      POSTGRES_DB: lab
    volumes:
      - dbdata:/var/lib/postgresql/data

volumes:
  dbdata:
```

Bind the application to localhost when the lab does not need external access.

---

# 63. Secure-Code Review Heuristics

When reading code, develop a fixed scan order.

## Heuristic 1: Find sinks

Search for:

```text
SQL execution
shell execution
HTML insertion
template rendering
file access
network requests
deserialization
cryptographic verification
privileged state changes
```

Then trace backward to the source.

## Heuristic 2: Find sources

Typical sources:

```text
HTTP parameters
headers
cookies
JSON bodies
uploaded files
webhooks
message queues
external APIs
mobile client state
blockchain calldata
```

## Heuristic 3: Find authority decisions

Search for:

```text
require_authenticated
is_admin
role
tenant_id
owner_id
permission
scope
capability
policy
```

Ask whether the value was trusted from the client or independently derived.

## Heuristic 4: Find state changes

Search for:

```text
UPDATE
INSERT
DELETE
transfer
withdraw
approve
publish
activate
deactivate
link
unlink
reset
```

These are high-value security boundaries.

## Heuristic 5: Find concurrency boundaries

Search for:

```text
read -> check -> write
retry
queue
job
cache
distributed lock
transaction
idempotency
unique constraint
```

---

# 64. Tool Selection by Question

| Question | Tools |
|---|---|
| What requests does the browser send? | Browser DevTools, Burp, ZAP, mitmproxy |
| What endpoints exist? | Browser, OpenAPI, ZAP, authorized discovery tools |
| Is object authorization correct? | Burp/ZAP + paired test accounts + API tests |
| Is an SQL sink reachable? | Code review, Semgrep/CodeQL, local test app |
| Does a parser behave unexpectedly? | Unit/fuzz tests, Burp/ZAP, property testing |
| Is a native parser memory-safe? | ASan/UBSan, AFL++, libFuzzer |
| Is a container overprivileged? | Trivy, Docker inspection, Kubernetes policy tooling |
| What mobile code actually runs? | Frida, jadx, MobSF, adb |
| What does a binary do? | Ghidra, radare2, debugger |
| Are Solidity invariants preserved? | Foundry, Echidna, Slither |
| Does a dependency have known CVEs? | OSV-Scanner, Trivy, Grype |

The important principle is:

```text
Question first
    |
    v
Evidence needed
    |
    v
Choose tool
```

Do not start with a tool and try to invent a reason to use it.

---

# 65. Example End-to-End Secure Review

Consider this endpoint:

```http
POST /api/v1/transfer
Authorization: Bearer <token>
Content-Type: application/json

{
  "from_account": "acct_123",
  "to_account": "acct_456",
  "amount": 500
}
```

## Step 1: Identity

```text
Who owns acct_123?
Who is the caller?
```

## Step 2: Authorization

```text
caller == account owner?
caller allowed to transfer?
recipient allowed?
```

## Step 3: Input

```text
amount numeric?
amount > 0?
amount within limits?
account IDs valid?
```

## Step 4: State

```text
balance sufficient?
account active?
not locked?
```

## Step 5: Concurrency

```text
Can two transfers spend the same balance?
```

## Step 6: Audit

```text
Is there a transaction record?
Can audit data itself be forged?
```

## Step 7: Idempotency

```text
Can network retries create two transfers?
```

## Secure service sketch

```python
class TransferService:
    def transfer(self, principal, command):
        validate_transfer_input(command)

        with self.db.transaction():
            source = self.db.lock_account(command.from_account)
            destination = self.db.lock_account(command.to_account)

            self.authz.require_transfer(principal, source)
            self.policy.require_active(source)
            self.policy.require_positive_amount(command.amount)
            self.policy.require_sufficient_funds(source, command.amount)

            self.db.debit(source.id, command.amount)
            self.db.credit(destination.id, command.amount)

            self.db.record_transfer(
                actor=principal.user_id,
                source=source.id,
                destination=destination.id,
                amount=command.amount,
            )
```

This implementation is still only a sketch. Production financial systems require careful transactional design, reconciliation, idempotency, fraud controls, ledger integrity, and operational review.

---

# 66. From Vulnerability to Root Cause

When a test fails, classify the root cause at the earliest layer that actually controls the security property.

```text
Observed behavior
      |
      v
Parser mistake?
      |
      +-- yes --> parsing root cause
      |
      no
      v
Authentication mistake?
      |
      +-- yes --> identity root cause
      |
      no
      v
Authorization mistake?
      |
      +-- yes --> policy root cause
      |
      no
      v
State / transaction mistake?
      |
      +-- yes --> business/concurrency root cause
      |
      no
      v
Infrastructure / dependency assumption?
```

This prevents reports that merely say "endpoint is vulnerable" without identifying why.

---

# 67. Advanced Mental Models

## 67.1 The authority equation

A useful conceptual equation is:

```text
Effective authority
  =
  authenticated identity
  ∩
  role/permissions
  ∩
  resource ownership
  ∩
  tenant boundary
  ∩
  workflow state
  ∩
  contextual policy
```

If any dimension is missing from the check, ask whether the operation becomes broader than intended.

## 67.2 The parser differential model

```text
Input bytes
   |
   +--> Parser A sees structure X
   |
   +--> Parser B sees structure Y
   |
   v
Different security decisions
```

This mental model is useful for:

- proxies
- caches
- JSON parsers
- XML parsers
- URL parsing
- multipart parsers
- authentication middleware

## 67.3 The confused-deputy model

A privileged server may accidentally use its own authority to perform an action on behalf of a low-privileged user.

```text
Low-privileged user
        |
        v
Application service
        |
        | privileged credential
        v
Powerful backend
```

Ask:

> Is the privileged component applying the caller's authorization context, or merely its own credentials?

## 67.4 The ambient-authority model

Whenever a process automatically possesses credentials, ask whether an attacker-controlled input can cause the process to spend that authority somewhere unintended.

Examples include:

```text
browser cookies
server API keys
cloud workload identity
service account tokens
filesystem permissions
smart-contract privileges
```

---

# 68. Recommended Learning Sequence by Skill

## If you want strong web/API fundamentals

```text
HTTP
  -> sessions
  -> authentication
  -> authorization
  -> IDOR/BOLA
  -> OAuth/OIDC
  -> business logic
  -> SQLi
  -> XSS
  -> SSRF
  -> race conditions
  -> GraphQL
  -> cloud IAM
```

## If you want exploit development

```text
C
  -> memory model
  -> assembly
  -> debugger
  -> ABI
  -> mitigations
  -> fuzzing
  -> reverse engineering
  -> browser/mobile/native internals
```

## If you want smart-contract research

```text
Solidity
  -> EVM
  -> storage/layout
  -> calls/delegatecall
  -> ERC standards
  -> Foundry
  -> invariants
  -> fuzzing
  -> oracles
  -> lending
  -> DEXs
  -> bridges
  -> governance
```

## If you want security engineering rather than only bug hunting

Add:

```text
threat modeling
secure architecture
identity design
cryptography fundamentals
secure SDLC
CI/CD security
logging/monitoring
incident response
risk management
```

---

# 69. Practical Daily Practice

A productive 90-minute session can be:

```text
15 min  Read architecture/code
20 min  Identify one security invariant
20 min  Build one test
15 min  Diagnose the root cause
15 min  Implement/fix locally
05 min  Write regression test
```

The most important habit is to keep a notebook containing:

```text
Observation
Hypothesis
Test
Evidence
Conclusion
Root cause
Fix
Regression test
```

After enough repetitions, the process becomes an automatic reasoning loop.

---

# 70. Final Master Checklist

Before declaring a feature secure, ask:

```text
IDENTITY
[ ] Who is the caller?
[ ] How was identity established?
[ ] Can the client influence identity claims?

AUTHENTICATION
[ ] Can login state be forged?
[ ] Are recovery paths secure?
[ ] Is MFA state enforced server-side?

AUTHORIZATION
[ ] Is the operation authorized?
[ ] Is the exact object authorized?
[ ] Is tenant context enforced?
[ ] Are nested resources checked?

INPUT
[ ] What can the caller control?
[ ] Which parser consumes it?
[ ] Can data become code/query/path/URL?
[ ] Are sizes and types constrained?

STATE
[ ] What state transition occurs?
[ ] Are invalid transitions rejected?
[ ] Are retries safe?
[ ] Is the operation idempotent?

CONCURRENCY
[ ] Can two requests pass a check simultaneously?
[ ] Is there a transaction/lock/unique constraint?

DEPENDENCIES
[ ] Does the server trust another service?
[ ] Is that service authenticated?
[ ] Is caller context preserved?

INFRASTRUCTURE
[ ] What happens if this service is compromised?
[ ] Can it access secrets?
[ ] Can it reach internal networks?
[ ] Is IAM least privilege?

OUTPUT
[ ] Is untrusted data encoded for the correct context?
[ ] Can sensitive output be cached/logged accidentally?

IMPACT
[ ] What is the worst directly demonstrated effect?
[ ] What is the blast radius?
[ ] What privileges are required?
[ ] Is exploitation reliable?

REMEDIATION
[ ] Is the control server-side?
[ ] Is it enforced at the correct boundary?
[ ] Is a regression test present?
```

---

# 71. Closing Principle

A payload is a tool. A methodology is a capability.

The durable skill is to move through this chain quickly and accurately:

```text
SYSTEM
  |
  v
ASSET
  |
  v
IDENTITY
  |
  v
AUTHORITY
  |
  v
TRUST BOUNDARY
  |
  v
INPUT / STATE
  |
  v
INVARIANT
  |
  v
FAILURE MODE
  |
  v
EXPLOITABILITY
  |
  v
IMPACT
  |
  v
ROOT CAUSE
  |
  v
FIX
  |
  v
REGRESSION TEST
```

When that chain becomes automatic, learning a new framework, API style, language, cloud service, mobile stack, or protocol becomes much easier because you already know what to look for.

The goal is not to memorize the largest number of payloads. The goal is to recognize where **authority, trust, parser behavior, and state transitions do not line up with the security invariants of the system**.
