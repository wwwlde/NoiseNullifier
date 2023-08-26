# 🚫🔊 NoiseNullifier

NoiseNullifier reduces alert notification noise. No more being bombarded on your chat or phone. Peace at last!

NoiseNullifier isn't about introducing complex operations or intelligence into your alert management system. Instead, it's about simplifying processes, minimizing distractions, and allowing teams to focus on resolving incidents without the constant buzz of ongoing alerts.

## Introduction

NoiseNullifier is a straightforward solution designed to streamline the response mechanism between PagerDuty and AlertManager. Upon receiving an acknowledgment event from PagerDuty via Webhook v3, NoiseNullifier initiates the process to silence the alerts associated with that incident in AlertManager.

---

## 🌟 Features

- **Webhook Ready**: Designed to easily connect with PagerDuty through webhooks.
- **Silence Overload**: No more being bombarded on your chat or phone. Peace at last!
- **Logging**: Detailed logs allow you to track the life cycle of alerts and debug with ease.
- **Incident-Driven Silencing**: As soon as PagerDuty acknowledges an incident, NoiseNullifier takes action, sending silence commands to the relevant AlertManager, ensuring all related alerts are temporarily silenced.
- **Simplified Configuration**: Setting up NoiseNullifier is a breeze. With your PagerDuty API key and webhook secret configured via environment variables, you're all set to go!

---

# 🚀 Prerequisites

- **Direct Connectivity**: It's essential that NoiseNullifier can access all AlertManagers associated with your PagerDuty setup directly.
- **Webhook Compatibility**: NoiseNullifier is optimized for PagerDuty's Webhook v3. Ensure your webhook is set to account-scoped to guarantee smooth operation.

---

# 🐢 Getting Started

With the following command you can download and build this app.

## Prerequisites

- Go 1.21 environment

## Installation Steps:

1 **Clone the Repository**:

```bash
   git clone https://github.com/wwwlde/NoiseNullifier.git
```

2 **Move to Directory**:

```bash
   cd NoiseNullifier
```

3 **Build**:

```bash
   CGO_ENABLED=0 GOOS=linux go build -o ./NoiseNullifier --ldflags '-extldflags "-static"' .
```
4 **Profit!**

---

# ⚙ Configuration

Setup Configuration:

1. PagerDuty API Key: This key allows NoiseNullifier to interface seamlessly with your PagerDuty environment.
2. Webhook Secret: Add this to your environment configuration to ensure secure communication between PagerDuty and NoiseNullifier.
3. Direct Access: Make sure that NoiseNullifier can freely communicate with all AlertManagers integrated with PagerDuty.
4. Webhook Versioning: Activate Webhook v3 on PagerDuty, ensuring it's set to account-scoped.

To configure your PagerDuty provisioning key and secret, you can use environment variables.

| Key          | Description                 |
|--------------|-----------------------------|
| `PD_SECRET`  | Your PagerDuty secret key.  |
| `PD_APIKEY`  | Your PagerDuty API key.     |

**Note**: Note for the developer, every time you change enformcement variable you should restart app. This will help you to keep track of your current state.

---

# ▶️ Running

Execute the following:

```bash
$ ./NoiseNullifier-linux -h
This tool acts as a bridge, converting PagerDuty webhook events to AlertManager silences.

Usage:
  NoiseNullifier [flags]

Flags:
  -h, --help   help for NoiseNullifier
```

Upon running, the service will start listening on port 8080 for incoming webhooks.

---

# 🤝 Contributing

All contributions are welcomed!

---

# 📜 License

GNU General Public License v3.0 (GPL-3.0)
