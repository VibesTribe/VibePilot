VibePilot Complete Disaster Recovery Guide
============================================

If your laptop dies, catches fire, or the hard drive corrupts,
this document is EVERYTHING you need to restore the entire system
on a fresh machine from scratch.

Last updated: May 24 2026
Machine: Lenovo ThinkPad X220 (i5-2520M, 16GB RAM, 916GB SSD/HDD)
OS: Linux Mint 22.3 (based on Ubuntu 24.04)

==================================================================
WHAT IS ON GITHUB (your off-site backups)
==================================================================

4 repositories under github.com/VibesTribe/:

1. VibePilot (85+ commits) - Main repo
   The Go governor, PostgreSQL migrations, agent configs, prompts,
   pipeline definitions, model configs. This is the ENGINE.

2. vibeflow (149+ commits) - Dashboard
   Next.js dashboard for monitoring tasks, ROI, system health.
   Deployed to Vercel. Push to main = auto-deploy.

3. knowledgebase (98+ commits) - KB + DB backup
   Research docs, decision records, system maps, skills database.
   Also has a 'backups' branch with PostgreSQL dump (split into
   parts under 90MB each for GitHub's 100MB file limit).

4. vibes-agent-context (14+ commits) - Hermes memory
   Agent memory files (MEMORY.md, USER.md) synced every 30 min.
   Contains cross-session knowledge, user preferences, system facts.

==================================================================
STEP 1: GET A MACHINE
==================================================================

Any Linux machine with:
- 16GB+ RAM (governor + postgres + chrome + hermes)
- 200GB+ disk (postgres alone is 900MB, db-backups ~180MB)
- x86_64 CPU (Go binary is amd64)
- Internet access

Install Linux Mint 22.x or Ubuntu 24.04 LTS.
Create user: vibes

==================================================================
STEP 2: INSTALL SYSTEM DEPENDENCIES
==================================================================

sudo apt update && sudo apt upgrade -y
sudo apt install -y \
  git curl wget build-essential \
  postgresql-16 postgresql-client-16 \
  python3 python3-pip python3-venv \
  google-chrome-stable \
  jq

# Go 1.24+
wget https://go.dev/dl/go1.24.3.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.3.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:~/go/bin' >> ~/.bashrc
source ~/.bashrc

# Node.js 24 via nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
source ~/.bashrc
nvm install 24

# Vercel CLI (for dashboard deployment)
npm install -g vercel

==================================================================
STEP 3: SET UP POSTGRESQL
==================================================================

# Create the database and user
sudo -u postgres createuser --createdb vibes
sudo -u postgres createdb vibepilot -O vibes

# Enable local socket auth (no password needed)
# Edit /etc/postgresql/16/main/pg_hba.conf:
#   local   all   vibes   peer
sudo systemctl restart postgresql

# Verify connection
psql -d vibepilot -c "SELECT version();"

==================================================================
STEP 4: CLONE ALL REPOS
==================================================================

mkdir -p ~/github
cd ~/github

# You need a GitHub Personal Access Token (PAT) with repo access
# Store it once:
git config --global credential.helper store
echo "https://VibesTribe:YOUR_GITHUB_TOKEN@github.com" > ~/.git-credentials
git config --global user.name "VibePilot Server"
git config --global user.email "vibesagentai@gmail.com"

# Clone all 4 repos
git clone https://github.com/VibesTribe/VibePilot.git ~/VibePilot
git clone https://github.com/VibesTribe/vibeflow.git ~/vibeflow
git clone https://github.com/VibesTribe/knowledgebase.git ~/knowledgebase
git clone https://github.com/VibesTribe/vibes-agent-context.git ~/vibes-agent-context

# Create the running copy (governor reads from ~/vibepilot/)
cp -r ~/VibePilot ~/vibepilot

# Create the DB backups directory
mkdir -p ~/db-backups
cd ~/db-backups
git init -b backups
git remote add origin https://github.com/VibesTribe/knowledgebase.git
git config pack.windowMemory 256m
git config pack.deltaCacheSize 128m
git config pack.threads 2

==================================================================
STEP 5: RESTORE POSTGRESQL DATABASE
==================================================================

# Pull the backup from GitHub
cd ~/db-backups
git fetch origin backups
git checkout origin/backups -- .

# You should see files like: vibepilot-part-aa.gz, vibepilot-part-ab.gz, etc. (7 parts)
# Reassemble and restore:
cat vibepilot-part-*.gz | gunzip | psql -d vibepilot

# Verify restoration
psql -d vibepilot -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
# Should show 104+ tables

# Rebuild KB indexes (these are excluded from backup to save space)
# See Step 10 for the sync command

==================================================================
STEP 6: BUILD THE GO GOVERNOR
==================================================================

cd ~/vibepilot/governor
go build -o governor ./cmd/governor
sudo cp governor /usr/local/bin/vibepilot-governor

# Verify
vibepilot-governor --help || echo "Binary works (may not have --help flag)"

==================================================================
STEP 7: SET UP HERMES AGENT
==================================================================

# Create hermes directory structure
mkdir -p ~/.hermes/memories ~/.hermes/skills ~/.hermes/cron

# Clone hermes-agent (the engine)
cd ~/.hermes
git clone https://github.com/outsourc-e/hermes-agent.git
cd hermes-agent

# Create venv and install dependencies
python3 -m venv venv
source venv/bin/activate
pip install -e .
deactivate

# RESTORE HERMES CONFIG FROM GITHUB
cd ~/vibes-agent-context

# Config files (model routing, providers, fallbacks)
cp hermes-config/config.yaml ~/.hermes/
cp hermes-config/auth.json ~/.hermes/

# Cron jobs (tokenfinder, memory sync, KB sync, etc.)
cp hermes-config/cron/jobs.json ~/.hermes/cron/

# Skills (184 skill files across 31 categories)
# This is your accumulated agent knowledge
rsync -a skills/ ~/.hermes/skills/

# Memory (cross-session knowledge, user preferences)
cp memories/MEMORY.md ~/.hermes/memories/
cp memories/USER.md ~/.hermes/memories/

# Create .env from template and fill in your API keys
cp hermes-config/.env.template ~/.hermes/.env
nano ~/.hermes/.env  # Fill in all the *** values with real keys
# See "SECRETS YOU NEED" section below

# Verify Hermes can start
~/.hermes/hermes-agent/venv/bin/python -m hermes_cli.main --help

==================================================================
STEP 8: SET UP SYSTEMD SERVICES
==================================================================

# Create systemd user directory
mkdir -p ~/.config/systemd/user

# Create service files (see "Service Files" section below)

# HERMES GATEWAY
cat > ~/.config/systemd/user/hermes-gateway.service << 'EOF'
[Unit]
Description=Hermes Agent Gateway - Messaging Platform Integration
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=0

[Service]
Type=simple
ExecStart=/home/vibes/.hermes/hermes-agent/venv/bin/python -m hermes_cli.main gateway run --replace
WorkingDirectory=/home/vibes/.hermes/hermes-agent
Environment="PATH=/home/vibes/.hermes/hermes-agent/venv/bin:/home/vibes/.hermes/hermes-agent/node_modules/.bin:/home/vibes/.nvm/versions/node/v24.14.1/bin:/home/vibes/.local/bin:/home/vibes/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="VIRTUAL_ENV=/home/vibes/.hermes/hermes-agent/venv"
Environment="HERMES_HOME=/home/vibes/.hermes"
Restart=always
RestartSec=60
RestartMaxDelaySec=300
RestartSteps=5
RestartForceExitStatus=75
KillMode=mixed
KillSignal=SIGTERM
ExecReload=/bin/kill -USR1 $MAINPID
TimeoutStopSec=90
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
EOF

# HERMES GATEWAY OVERRIDE (env file)
mkdir -p ~/.config/systemd/user/hermes-gateway.service.d
cat > ~/.config/systemd/user/hermes-gateway.service.d/override.conf << 'EOF'
[Service]
EnvironmentFile=/home/vibes/.hermes/.env
EOF

# VIBEPILOT GOVERNOR
cat > ~/.config/systemd/user/vibepilot-governor.service << 'EOF'
[Unit]
Description=VibePilot Go Governor Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/home/vibes/vibepilot/governor
ExecStart=/home/vibes/vibepilot/governor/governor
ExecStartPre=-/bin/bash -c 'pkill -f "./governor" || true'
PIDFile=/home/vibes/.config/vibepilot/governor.pid
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=vibepilot-governor
Environment=PATH=/home/vibes/go/bin:/usr/local/bin:/usr/bin:/bin
Environment=HOME=/home/vibes

[Install]
WantedBy=default.target
EOF

# GOVERNOR OVERRIDE (secrets)
mkdir -p ~/.config/systemd/user/vibepilot-governor.service.d
cat > ~/.config/systemd/user/vibepilot-governor.service.d/override.conf << 'EOF'
[Service]
Restart=always
Environment="DATABASE_URL=postgres://vibes@/vibepilot?host=/var/run/postgresql"
Environment="SUPABASE_URL=YOUR_SUPABASE_URL"
Environment="SUPABASE_SERVICE_KEY=YOUR_SUPABASE_KEY"
Environment="VAULT_KEY=YOUR_VAULT_KEY"
Environment="GOVERNOR_CONFIG_DIR=/home/vibes/vibepilot/governor/config"
Environment="GOVERNOR_PROMPTS_DIR=/home/vibes/vibepilot/prompts"
Environment="HOME=/home/vibes"
Environment="PATH=/home/vibes/.local/bin:/usr/local/bin:/usr/bin:/bin"
Environment="REPO_PATH=/home/vibes/vibepilot"
Environment="GOVERNOR_ADMIN_TOKEN=YOUR_ADMIN_TOKEN"
EOF

# KNOWLEDGE BASE SERVER
cat > ~/.config/systemd/user/knowledgebase-server.service << 'EOF'
[Unit]
Description=Knowledge Hub API Server (Flask)
After=network.target

[Service]
Type=simple
WorkingDirectory=/home/vibes/knowledgebase
ExecStart=/usr/bin/python3 /home/vibes/knowledgebase/server.py
Environment=KNOWLEDGE_HUB_PORT=8888
Environment=KNOWLEDGE_HUB_HOST=127.0.0.1
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

# Enable and start all services
systemctl --user daemon-reload
systemctl --user enable hermes-gateway knowledgebase-server vibepilot-governor
systemctl --user start hermes-gateway knowledgebase-server vibepilot-governor

# Verify all running
systemctl --user status hermes-gateway knowledgebase-server vibepilot-governor

==================================================================
STEP 9: SET UP CRON JOBS
==================================================================

crontab -e

# Paste these lines:

# Hourly PostgreSQL backup to GitHub
0 * * * * /home/vibes/db-backups/split-backup.sh >> /tmp/pg-dump.log 2>&1

# Daily context sync
0 2 * * * bash /home/vibes/VibePilot/.context/sync.sh >> /home/vibes/.local/state/context-sync.log 2>&1

# Daily KB full sync (rebuilds indexes, embeddings)
0 3 * * * /home/vibes/knowledgebase/scripts/cron-sync.sh >> /tmp/kb-cron.log 2>&1

# Daily model health check
30 1 * * * /home/vibes/vibepilot/governor/scripts/daily_model_health.py >> /tmp/model-health.log 2>&1

# Cooldown reactivation every 5 minutes
*/5 * * * * python3 /home/vibes/knowledgebase/scripts/reactivate_cooldowns.py >> /tmp/reactivate_cooldowns.log 2>&1

# Daily analyst diagnosis
0 5 * * * /home/vibes/vibepilot/scripts/analyst_cron.sh >> /tmp/analyst.log 2>&1

==================================================================
STEP 10: SET UP KNOWLEDGE BASE
==================================================================

cd ~/knowledgebase

# Install Python dependencies
pip3 install --user flask psycopg psycopg-binary psycopg2-binary

# Install jcodemunch and jdocmunch (code/doc indexing)
pip3 install --user jcodemunch-mcp jdocmunch-mcp

# Rebuild KB indexes from source repos
python3 scripts/sync_all.py

# Start the KB server (or it starts via systemd)
# Accessible at http://localhost:8888

==================================================================
STEP 11: SET UP CHROME (for browser automation)
==================================================================

# Chrome is used for CDP-based browser automation
google-chrome --remote-debugging-port=9222 \
  --user-data-dir=/home/vibes/.config/chrome-debug \
  --headless=new \
  --disable-gpu \
  --no-sandbox &

# Verify CDP
curl http://127.0.0.1:9222/json/version

==================================================================
STEP 12: DEPLOY DASHBOARD TO VERCEL
==================================================================

cd ~/vibeflow

# Build locally first to verify
npm install
npm run build

# Deploy to Vercel
vercel --prod

# Or link to existing project:
vercel link
vercel --prod

==================================================================
STEP 13: RESTORE HERMES MEMORY AND SKILLS
==================================================================

This step is now part of Step 7 (Hermes setup).
The vibes-agent-context repo contains:
  memories/MEMORY.md    -- agent cross-session knowledge
  memories/USER.md      -- user preferences and profile
  skills/ (184 files)   -- all agent skill files
  hermes-config/        -- config.yaml, auth.json, cron/jobs.json

These are auto-synced every 30 minutes via sync_memories.sh.
On a fresh restore, just copy them in Step 7.

==================================================================
SECRETS YOU NEED (NOT in GitHub)
==================================================================

These must be manually restored or re-created:

1. GitHub Personal Access Token
   - Stored in ~/.git-credentials
   - Needs repo, workflow access
   - Generate at: github.com/settings/tokens

2. Gemini API Keys (4x free tier)
   - GEMINI_KEY_1 through GEMINI_KEY_4
   - In ~/.hermes/.env
   - Generate at: aistudio.google.com/apikey

3. OpenRouter API Key
   - OPENROUTER_API_KEY in ~/.hermes/.env
   - Generate at: openrouter.ai/keys

4. Groq API Key
   - GROQ_API_KEY in ~/.hermes/.env
   - Generate at: console.groq.com

5. NVIDIA API Key
   - NVIDIA_API_KEY in ~/.hermes/.env
   - Generate at: build.nvidia.com

6. Z.AI / GLM Keys
   - ZAI_API_KEY, ZAI_GLM51_KEY in ~/.hermes/.env
   - From z.ai subscription

7. DeepSeek Key
   - DEEPSEEK_V4_FLASH_KEY, DEEPSEEK_API_KEY in ~/.hermes/.env
   - Generate at: platform.deepseek.com

8. VibePilot Vault Key
   - VAULT_KEY in governor systemd override
   - Used to encrypt/decrypt secrets_vault table
   - IF LOST: encrypted secrets are unrecoverable

9. Supabase Credentials
   - SUPABASE_URL and SUPABASE_SERVICE_KEY
   - In governor systemd override
   - From: supabase.com dashboard

10. Vercel Token
    - For dashboard deployment
    - Generate at: vercel.com/account/tokens

11. Telegram Bot Token (if re-enabled)
    - TELEGRAM_BOT_TOKEN in ~/.hermes/.env
    - From @BotFather on Telegram

12. Gmail App Password (for email alerts)
    - VIBEPILOT_GMAIL_EMAIL, VIBEPILOT_GMAIL_PASSWORD
    - In ~/.hermes/.env

==================================================================
COMPLETE FILE MAP
==================================================================

~/.hermes/
  .env                          -- API keys and provider config
  auth.json                     -- Credential pools for Hermes
  config.yaml                   -- Main Hermes config (model, providers, fallbacks)
  memories/
    MEMORY.md                   -- Agent cross-session knowledge
    USER.md                     -- User preferences and profile
  skills/                       -- 34 skill directories
  cron/jobs.json                -- Hermes cron job definitions
  hermes-agent/                 -- Hermes agent source (git repo)

~/VibePilot/                    -- Dev copy (push to this one)
~/vibeflow/                     -- Dashboard (push = auto-deploy to Vercel)
~/knowledgebase/                -- KB docs, research, scripts
~/vibes-agent-context/          -- Hermes memory backup repo
~/vibepilot/                    -- Running copy (governor reads from here)
~/db-backups/                   -- PostgreSQL dump on 'backups' branch

~/.config/systemd/user/
  hermes-gateway.service        -- Hermes gateway service
  hermes-gateway.service.d/override.conf
  vibepilot-governor.service    -- Governor service
  vibepilot-governor.service.d/override.conf  -- secrets
  knowledgebase-server.service  -- KB Flask API

==================================================================
VERIFICATION CHECKLIST
==================================================================

After restore, verify each system:

1. PostgreSQL
   psql -d vibepilot -c "SELECT count(*) FROM model_catalog;"
   psql -d vibepilot -c "SELECT status, COUNT(*) FROM project_todos GROUP BY status;"

2. Governor
   systemctl --user status vibepilot-governor
   journalctl --user -u vibepilot-governor --since "5 min ago"

3. Hermes Gateway
   systemctl --user status hermes-gateway
   curl http://localhost:8642/health 2>/dev/null

4. Knowledge Base Server
   systemctl --user status knowledgebase-server
   curl http://localhost:8888/api/health 2>/dev/null

5. Chrome CDP
   curl http://127.0.0.1:9222/json/version

6. Dashboard (Vercel)
   curl -s https://your-vercel-app.vercel.app | head -5

7. Git push test
   cd ~/VibePilot && git push --dry-run origin main

==================================================================
BACKUP SCHEDULE (what runs automatically)
==================================================================

Every hour:     PostgreSQL dump -> GitHub (backups branch)
Every 30 min:   Hermes memory -> GitHub (vibes-agent-context)
Every 30 min:   TokenFinder model scan
Daily 2am:      Context sync
Daily 3am:      KB full sync (reindex + embed)
Daily 1:30am:   Model health check
Every 5 min:    Cooldown reactivation

All code and config is in GitHub.
The ONLY thing not automated is the secrets list above.

Keep this document in ~/VibePilot/docs/DISASTER-RECOVERY.md
and also print/save a copy OFFLINE (USB stick, phone, email to self).
If your laptop dies, you need the secrets list and this guide.

The secrets that CANNOT be recovered if lost:
- VAULT_KEY (encrypts secrets_vault table)
- GitHub PAT (can regenerate)
- All API keys (can regenerate from provider consoles)

TIP: Store VAULT_KEY in a password manager or write it on paper.
Lose it and the encrypted secrets in the DB are gone forever.
