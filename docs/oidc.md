---
layout: default
title: "Single Sign-On (OIDC)"
nav_order: 6
description: "Signing in to aura with an OpenID Connect identity provider."
permalink: /oidc
---

# Single Sign-On (OIDC)

aura can hand sign-in over to an OpenID Connect provider such as Authentik, Keycloak, Auth0, Okta or Google. It acts as a **confidential client** using the authorization code flow with PKCE: the code exchange and token verification happen in the backend, and the browser only ever receives an `HttpOnly` session cookie.

Password sign-in and SSO are independent. Leaving the password hash configured alongside OIDC gives you a way in when the provider is unavailable.

---

## 1. Register aura with your provider

Create a **confidential** (client secret) application using the **authorization code** flow, and set the redirect URI to your external aura URL followed by `/api/auth/oidc/callback`:

```
https://aura.example.com/api/auth/oidc/callback
```

If you enable [single logout](#end-provider-session-on-logout), also allow this post-logout redirect URI:

```
https://aura.example.com/login
```

Provider-specific notes:

| Provider  | Issuer URL |
| --------- | ---------- |
| Authentik | `https://auth.example.com/application/o/<application-slug>/` |
| Keycloak  | `https://keycloak.example.com/realms/<realm>` |
| Auth0     | `https://<tenant>.auth0.com/` |
| Google    | `https://accounts.google.com` |

To receive groups, the provider must be configured to emit them: Authentik needs the `groups` scope mapping on the application, and Keycloak needs a group membership mapper adding the `groups` claim to the ID token.

---

## 2. Configure aura

Through **Settings → Authentication → Single Sign-On**, or directly in `config.yaml`:

```yaml
Auth:
  Enabled: true
  Password: YOUR_ARGON2ID_HASH_HERE # optional once OIDC is enabled, but keep it for break-glass access
  OIDC:
    Enabled: true
    IssuerURL: https://auth.example.com/application/o/aura/
    ClientID: aura
    ClientSecret: YOUR_CLIENT_SECRET_HERE
    RedirectURL: https://aura.example.com/api/auth/oidc/callback
    Scopes: [openid, profile, email, groups]
    GroupsClaim: groups
    AllowedGroups: [aura-users]
    ButtonLabel: "Sign in with Authentik"
    RPInitiatedLogout: true
```

### Enabled

- **Default**: `false`
- **Description**: Whether the login page offers SSO. `Auth.Enabled` must also be `true`.

### IssuerURL

- **Description**: The provider's issuer. aura appends `/.well-known/openid-configuration` to discover the endpoints and signing keys, so no other URL needs configuring.
- **Note**: Discovery happens the first time someone signs in, not at startup — aura still boots when the provider is down.

### ClientID / ClientSecret

- **Description**: The credentials issued when you registered the application.
- **Note**: The secret is masked (`***abcd`) in the settings UI and in API responses. Saving the masked value leaves the stored secret untouched; paste a new value to replace it.

### RedirectURL

- **Description**: The callback registered with your provider, as an absolute URL.
- **Details**: This is never derived from the incoming request — the `Host` header is client-controlled, so guessing it would let a malicious host redirect sign-ins elsewhere. Behind a reverse proxy, use the external URL your users type.

### Scopes

- **Default**: `openid, profile, email`
- **Description**: Scopes to request. `openid` is always included. Add whatever scope carries group membership if you intend to restrict by group (`groups` for Authentik and Keycloak).

### GroupsClaim

- **Default**: `groups`
- **Description**: Which ID token claim holds group membership. The value may be a list or a single string.

### AllowedGroups / AllowedEmails / AllowedSubjects

- **Default**: empty
- **Description**: Who may sign in.
- **Details**: With **every** list empty, any user your provider authenticates is allowed in — which is the right setting when the provider already controls who can open the application. With **any** list set, the user must match at least one entry across the three lists. Matching ignores case.

### ButtonLabel

- **Default**: `Sign in with SSO`
- **Description**: The label on the login page button.

### End Provider Session on Logout

- **Config key**: `RPInitiatedLogout`
- **Default**: `false`
- **Description**: After clearing the local session, send the browser to the provider's `end_session_endpoint` so it signs you out there too.
- **Details**: aura does not retain the provider's ID token, so the request identifies itself with `client_id` and `post_logout_redirect_uri` rather than `id_token_hint`. Providers that require the post-logout URL to be registered need `https://aura.example.com/login` allowlisted.

---

## Sessions

A successful sign-in issues aura's own session token, valid for 24 hours, in an `HttpOnly`, `SameSite=Lax` cookie. The `Secure` flag is set when the request arrives over HTTPS, directly or via `X-Forwarded-Proto`.

The session is **not** refreshed from the provider: revoking a user at the identity provider does not end an aura session already in progress; it takes effect within 24 hours, when the session expires and the user has to sign in again.

---

## Troubleshooting

Sign-in failures return you to the login page with a message. What each one means:

| Message | Cause |
| ------- | ----- |
| Single sign-on is not enabled | `Auth.Enabled` or `Auth.OIDC.Enabled` is `false` |
| The identity provider could not be reached or refused the sign-in | Discovery failed (wrong `IssuerURL`, DNS, TLS) or the provider denied the request |
| The sign-in attempt expired or did not match | More than 10 minutes elapsed, or cookies were dropped between the two requests |
| The identity provider rejected the authorization code | Wrong `ClientSecret`, or a `RedirectURL` that differs from the registered one |
| The identity token could not be verified | Signature, issuer, audience or nonce mismatch |
| Your account is not permitted to access aura | The user matched none of the configured allowlists |

The backend log carries the underlying reason for each — the login page deliberately shows a fixed message rather than the provider's own text.

**Locked out?** Set `Auth.Enabled: false` (or fix `Auth.Password`) in `config.yaml` and restart the container.
