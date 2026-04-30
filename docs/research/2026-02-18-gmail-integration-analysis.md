# Gmail Integration Research for VibePilot

**Date:** 2026-02-18  
**Researcher:** Kimi (System Researcher)  
**Tag:** VET (Council review recommended)  
**Priority:** HIGH  

---

## Use Cases Identified

### 1. Daily Digest Emails (Vibes → Human)
- Morning briefing: tasks completed, costs, platform status
- Evening summary: progress, next day preview
- Exception alerts: failures, limits reached, blockers

### 2. Review Notifications (Council → Human)
- Plan ready for review
- Architecture changes need approval
- Visual/UI changes awaiting human check

### 3. Agent Workflow Support (Courier Use Case)
- Web couriers receive chat URLs via email
- Agents can access "return to conversation" links
- Long-running task notifications

---

## Options Analysis

### Option A: Gmail API (OAuth 2.0)

**Implementation:**
```python
from googleapiclient.discovery import build
from google.oauth2.credentials import Credentials

# Send email
service = build('gmail', 'v1', credentials=creds)
message = create_message(sender, to, subject, body)
send_message(service, 'me', message)
```

**Pros:**
- Native Gmail integration
- Rich formatting (HTML emails)
- Thread support
- Label management
- Free tier: 1 billion quota units/day
- Supports sending AND reading

**Cons:**
- OAuth complexity (token refresh, consent screen)
- Google API dependencies
- Rate limits to respect
- Security scope (can read ALL email if not careful)

**Cost:** FREE (within limits)

---

### Option B: SMTP (Simple Mail Transfer Protocol)

**Implementation:**
```python
import smtplib
from email.mime.text import MIMEText

server = smtplib.SMTP('smtp.gmail.com', 587)
server.starttls()
server.login(email, app_password)
server.sendmail(from_addr, to_addr, msg.as_string())
```

**Pros:**
- Simpler than OAuth
- Works with any email provider
- No Google API dependencies
- Easy to test locally
- Well-documented

**Cons:**
- Requires "App Password" (less secure than OAuth)
- Google deprecating "less secure apps"
- No advanced Gmail features (labels, threads)
- Sending only (can't read inbox)

**Cost:** FREE

---

### Option C: SendGrid/Mailgun API

**Implementation:**
```python
import sendgrid
from sendgrid.helpers.mail import Mail

sg = sendgrid.SendGridAPIClient(api_key)
mail = Mail(from_email, to_email, subject, content)
response = sg.client.mail.send.post(request_body=mail.get())
```

**Pros:**
- High deliverability
- Analytics (open rates, clicks)
- Templates
- Webhooks for bounces
- Reliable infrastructure

**Cons:**
- Third-party dependency
- Cost at scale (though free tiers exist)
- Another API to manage
- Vendor lock-in (contradicts VibePilot principles)

**Cost:** 
- SendGrid: 100 emails/day free
- Mailgun: 5,000 emails/month free (3 months), then pay

---

### Option D: Local SMTP Server (Postfix/Exim)

**Pros:**
- Full control
- No external dependencies
- Sovereign (runs on your $5 Hetzner)

**Cons:**
- Complex setup
- Deliverability issues (spam filters)
- IP reputation management
- Not recommended for production

**Cost:** FREE (server resources only)

---

## Recommendation Matrix

| Criteria | Gmail API | SMTP | SendGrid | Local SMTP |
|----------|-----------|------|----------|------------|
| **Ease of setup** | Medium | Easy | Easy | Hard |
| **Security** | High (OAuth) | Medium | High | Low |
| **VibePilot alignment** | Medium | High | Low | High |
| **Cost** | FREE | FREE | Limited free | FREE |
| **Features** | High | Basic | High | Basic |
| **Read inbox?** | Yes | No | No | Yes |
| **Sovereignty** | Medium | High | Low | High |

---

## Recommended Approach: Hybrid

**Primary: Gmail API with OAuth (Sending)**
- Daily digests from Vibes
- Review notifications
- Rich HTML formatting

**Secondary: SMTP Fallback**
- If Gmail API fails
- Simpler backup option
- Quick testing

**Rationale:**
- Gmail API gives us reading capability (for courier use case)
- OAuth is the modern standard
- Free tier generous enough
- Can be made swappable (config-driven provider)

---

## Implementation Considerations

### 1. Security (CRITICAL)

**OAuth 2.0 Scopes:**
```
gmail.send          # Send only (minimum)
gmail.readonly      # Read emails (for courier)
gmail.modify        # Read + labels (not recommended)
gmail.metadata      # Headers only
```

**Recommendation:** 
- Default: `gmail.send` only
- Courier agents: Separate credential with `gmail.readonly`
- Store tokens in vault (not code)
- Refresh tokens automatically

### 2. Rate Limits

**Gmail API:**
- 1 billion quota units/day
- Per-method costs:
  - `send`: 100 units
  - `get`: 5 units
  - `list`: 5 units
- Daily digest: ~100 units/day (negligible)

### 3. Email Templates

**Daily Digest Template:**
```html
Subject: VibePilot Daily - [Date] - [X] Tasks Completed

## Summary
- Tasks completed: X
- Tokens used: Y
- Virtual cost: $Z
- Top performer: [Model]

## Platform Status
- ChatGPT: X/40 requests
- Claude: X/10 requests
- [Platform]: [Status]

## Alerts
- [Any subscription/credit warnings]

## Today's Focus
- [Active tasks]
- [Blockers if any]
```

### 4. Agent Workflow Integration

**For Courier Agents:**
1. Receive chat URL via email
2. Click link, complete task
3. Reply to email with results
4. Vibes parses response

**Or simpler:**
- Email contains task packet JSON
- Agent uploads result to Supabase directly
- Email just notification, not data channel

---

## Configuration Design

**vibepilot.yaml addition:**
```yaml
notifications:
  enabled: true
  provider: gmail_api  # or smtp, sendgrid
  
  gmail:
    credentials_path: /secure/gmail_credentials.json
    sender_email: vibes@vibepilot.local
    
  smtp_fallback:
    host: smtp.gmail.com
    port: 587
    username: ${SMTP_USER}
    password: ${SMTP_PASS}  # App password
    
  digest_schedule:
    morning: "08:00"  # UTC
    evening: "20:00"  # UTC
    
  alerts:
    credit_low: true
    limits_reached: true
    task_failures: true
```

---

## Council Review Questions

**VET Recommended because:**
1. **Security scope** - Email access is sensitive
2. **Vendor lock-in** - Gmail API dependency
3. **Architecture impact** - New credential type, new failure modes
4. **Privacy** - Daily digests contain project data

**Questions for Council:**
1. Should we support multiple providers (Gmail + SMTP + ?) for redundancy?
2. Is storing email content in Supabase acceptable or privacy risk?
3. Should daily digest be opt-in or default?
4. What level of detail in notifications? (Full task list vs summary?)
5. Fallback strategy if all notification methods fail?

---

## Next Steps (If Approved)

1. **SIMPLE:** Implement Gmail OAuth flow
2. **SIMPLE:** Create email templates
3. **SIMPLE:** Add to `vibepilot.yaml` config
4. **VET:** Security audit of credential storage
5. **SIMPLE:** Test daily digest generation
6. **VET:** Privacy review of email content

---

## Research Sources

- Gmail API Documentation: https://developers.google.com/gmail/api/guides/sending
- OAuth 2.0 Scopes: https://developers.google.com/identity/protocols/oauth2/scopes#gmail
- Rate Limits: https://developers.google.com/gmail/api/reference/quota
- SMTP vs API comparison: Various security blogs

---

## Final Recommendation

**Tag: VET**

Implement Gmail API with OAuth as primary, SMTP as fallback. Provides best feature set while maintaining VibePilot principles (config-driven, swappable, reversible).

Requires Council review for security scope decision before implementation.

---

*Analysis complete. Awaiting Council review for go/no-go.*
