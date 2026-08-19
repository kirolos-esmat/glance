<p align="center"><img src="docs/logo.png"></p>
<h1 align="center">Glance — Community Pro Edition</h1>
<p align="center">
  <a href="#installation">Install</a> •
  <a href="NEW_FEATURES.md">🚀 New Features (75+ PRs)</a> •
  <a href="custom-widgets/README.md">📦 Custom Widgets Lib</a> •
  <a href="docs/configuration.md#configuring-glance">Configuration</a> •
  <a href="https://discord.com/invite/7KQ7Xa9kJd">Discord</a> •
  <a href="https://github.com/sponsors/glanceapp">Sponsor</a>
</p>
<p align="center">
  <a href="https://github.com/glanceapp/community-widgets">Community widgets</a> •
  <a href="docs/preconfigured-pages.md">Preconfigured pages</a> •
  <a href="docs/themes.md">Themes</a>
</p>

<p align="center">A lightweight, highly customizable dashboard that displays<br> your feeds in a beautiful, streamlined interface — supercharged with <b>75+ community features</b>.</p>

![](docs/images/readme-main-image.png)

> [!NOTE]
> **🚀 Enhanced Community Build:** This repository includes **75+ merged pull requests** featuring OIDC SSO authentication, per-page access control, tabbed widget stacks, native Tautulli & TrueNAS widgets, Navidrome, Ghostfolio, Markdown, ICS calendar feeds, ControlD DNS, theme switcher, and a plug-and-play [**Custom Widgets Library**](custom-widgets/README.md). Read the full [**NEW_FEATURES.md**](NEW_FEATURES.md) for architectural blueprints and complete configuration examples.

---

## Features

### 🧩 Massive Widget Ecosystem
* **🎬 Tautulli:** Real-time Plex active stream counts, session details, friendly user names, and progress bars *(New - PR #1060)*
* **🗄️ TrueNAS:** Native TrueNAS Scale pool health status, system load average, uptime, and pending alerts *(New - PR #1060)*
* **📦 Custom Widgets Library:** Plug-and-play modular YAML widgets for Jellyfin dynamic sessions, Immich photos/storage, Radarr/Sonarr queues, Cloudflare tunnels, Tailscale, and Netbird *(New - `custom-widgets/`)*
* **🎵 Navidrome:** Display currently playing music, album covers, and artist metadata from your Subsonic/Navidrome server *(New)*
* **📈 Ghostfolio:** Live portfolio valuation, asset performance charts, and balances *(New)*
* **📝 Markdown:** Native markdown rendering and notes embedding directly on your dashboard *(New)*
* **📥 qBittorrent:** Active downloads, upload rates, and seeding ratio monitor *(New)*
* **📅 ICS Calendars:** Direct public `.ics` iCalendar URL subscriptions (Google, Apple, Outlook) *(Enhanced)*
* **🛡️ DNS Statistics:** Full analytics for ControlD, Pi-hole v6, AdGuard Home, Technitium, and Blocky *(Enhanced)*
* **🔍 Multi-Engine Search:** Built-in support for Brave Search, DuckDuckGo, Google, Kagi, Perplexity *(Enhanced)*
* **⚙️ Advanced Custom API:** RegEx replacement, array parameter handling, and range mapping *(Enhanced)*
* **RSS feeds** with conditional HTTP requests to save bandwidth
* **Subreddit & Reddit feeds** with thumbnail previews
* **Hacker News & Lobsters posts**
* **Weather forecasts** with hourly and multi-day metrics
* **YouTube channel uploads** with configurable sorting (posted vs updated)
* **Twitch channels** live status
* **Market prices** with net-change sorting and hover tooltips
* **Docker containers status** & server hardware stats
* [And many more...](docs/configuration.md#configuring-glance)

---

### 🔐 Enterprise-Grade Security & Access Management
* **OpenID Connect (OIDC) SSO:** Seamless authentication with Keycloak, Authelia, Authentik, PocketID, or Google.
* **Per-Page Access Control:** Restrict specific dashboards or sensitive widgets to designated user accounts (`allowed-users`).
* **Brute-force protection:** Automatic IP rate-limiting on failed login attempts.

---

### 🗂️ Dynamic UI & Tabbed Stacking
* **Widget Stacks / Tabs:** Conserve screen real estate by grouping multiple widgets into clickable tabs (`type: stack`).
* **Live Refresh Worker:** Dedicated background poller with configurable per-widget intervals and force-refresh endpoints.
* **Interactive Theme Switcher:** Select themes on the fly directly from the UI.
* **Mobile-First Layout:** Auto-expanding navigation and optimized touch layouts for phones and tablets.

---

### ⚡ Fast and Lightweight
* Low memory usage (<50MB RAM)
* Few dependencies, zero bloated JS frameworks
* Single small binary available for multiple OSs & architectures and lightweight Docker container
* Sub-second render times

![](docs/images/mobile-preview.png)

---

### 🎨 Themeable
Easily create your own theme by tweaking HSL color values or choose from one of the built-in presets (including dark, light, and *Shades of Purple*).

![](docs/images/themes-example.png)

<br>

## Configuration
Configuration is done through YAML files. To learn more about how the layout works, how to add more pages, and how to configure widgets, visit the [configuration documentation](docs/configuration.md#configuring-glance) and the [NEW_FEATURES blueprint](NEW_FEATURES.md).

<details>
<summary><strong>Preview example configuration file</strong></summary>
<br>

```yaml
server:
  base-url: https://dashboard.example.com
  proxied: true

theme:
  disable-picker: false # Interactive theme switcher

auth:
  secret-key: ${secret:session-key}
  oidc:
    issuer: https://auth.example.com/realms/home
    client-id: glance
    client-secret: ${secret:oidc-secret}

pages:
  - name: Home
    columns:
      - size: small
        widgets:
          - type: search
            search-engine: brave
            placeholder: "Search the web with Brave..."
          - type: clock
            hour-format: 24h

      - size: full
        widgets:
          # Tabbed Widget Stack
          - type: stack
            widgets:
              - type: calendar
                first-day-of-week: monday
                ics:
                  - https://calendar.google.com/calendar/ical/example/public/basic.ics
              - type: navidrome
                url: https://music.example.com
                user: admin
                pass: ${secret:music-pass}
              - type: markdown
                source: |
                  ### 🚀 Glance Pro Dashboard
                  All systems operational.

      - size: small
        widgets:
          - type: weather
            location: London, United Kingdom
            units: metric
          - type: dns-stats
            service: controld
            url: https://api.controld.com
            token: ${secret:controld-token}
```
</details>

<br>

## Installation

Choose one of the following methods:

<details>
<summary><strong>Docker compose using provided directory structure (recommended)</strong></summary>
<br>

Create a `docker-compose.yml` file with the following contents:

```yaml
services:
  glance:
    container_name: glance
    image: ghcr.io/kirolos-esmat/glance:latest
    restart: unless-stopped
    volumes:
      - ./config/glance.yml:/app/config/glance.yml:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - 8080:8080
```

When ready, run:

```bash
docker compose up -d
```

If you encounter any issues, check logs with:

```bash
docker compose logs -f
```

<hr>
</details>

<details>
<summary><strong>Manual binary installation</strong></summary>
<br>

To run the binary directly:

```bash
./glance --config /etc/glance.yml
```

<hr>
</details>

<br>

## Building from source

Requirements: [Go](https://go.dev/dl/) >= v1.23 or [Docker](https://docs.docker.com/engine/install/)

### Build Docker Image
```bash
docker build -t glance:latest .
```

### Build Go Binary
```bash
CGO_ENABLED=0 go build -o glance .
```

<br>

## Feature Catalog & Changelog
For the complete list and documentation of all 74 merged community pull requests, check out [**NEW_FEATURES.md**](NEW_FEATURES.md).

<br>

## Thank you

To all the original creators, maintainers, and community contributors who submitted PRs, issues, and themes. Your support makes Glance one of the best self-hosted dashboards available!
