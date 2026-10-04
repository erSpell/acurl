# Signing requests with Web Bot Auth

Spoofing a user agent string proves nothing: anyone can send `GPTBot`. Cryptographically
identified agents are where the web is heading. The Web Bot Auth proposal has agents sign every
request with [HTTP Message Signatures (RFC 9421)](https://www.rfc-editor.org/rfc/rfc9421)
using an Ed25519 key. They publish the public key at a well-known URL, so sites and CDNs can verify
who is calling.

acurl can sign its bot-profile requests the same way. You can use this to:

- **Test your verifier.** Check that your WAF, CDN or middleware accepts valid signatures and
  rejects unsigned or tampered requests.
- **Build your own agent.** Make sure your signing setup works before you ship it.

## 1. Generate a key

```bash
acurl keygen -o mybot.key
```

```
Wrote private key to mybot.key (keep it secret)
Key ID (JWK thumbprint): poqkLGiymh_W0uP6PZFw-dvez3QJT5SolqXBCW38r0U
Publish this at https://<your-domain>/.well-known/http-message-signatures-directory:
{
  "keys": [
    { "kty": "OKP", "crv": "Ed25519", "x": "...", "kid": "poqkLGiy..." }
  ]
}
```

Publish the JSON key directory at
`https://bot.example/.well-known/http-message-signatures-directory`.
To print it again later, run `acurl keygen --key mybot.key`.

## 2. Send signed requests

```bash
acurl --sign mybot.key --signature-agent https://bot.example https://shop.example/products/1
```

Every bot-profile request then carries:

```
Signature-Agent: "https://bot.example"
Signature-Input: sig1=("@authority" "signature-agent");created=1759400000;expires=1759400060;
                 keyid="poqkLGiy...";alg="ed25519";nonce="...";tag="web-bot-auth"
Signature: sig1=:...base64...:
```

- The signature covers `@authority` and `Signature-Agent`, and is valid for one minute.
- A random nonce is added to every request so verifiers can reject replays.
- The browser baseline is never signed. Signed rows show `signed` in the details column, and
  `"signed": true` appears in JSON output.

## Compatibility

The signing code is checked against the Ed25519 test vector in RFC 9421 Appendix B.2.6.
Web Bot Auth is still an IETF draft, so header details may change. Open an issue if a verifier
rejects acurl's signatures.

---

acurl is an open-source project by [tragent.io](https://tragent.io).
