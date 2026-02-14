# TEMPORARY FILE - Cron Job Setup Commands

**Delete this file after setting up cron jobs**

---

## Option 1: Daily Supabase Backup

Copy and paste this command:

```bash
crontab -e
```

Then add this line (runs daily at 2 AM):

```bash
0 2 * * * cd /home/mjlockboxsocial/vibepilot && ./scripts/backup_supabase.sh >> logs/backup.log 2>&1
```

Save and exit.

---

## Option 2: Create logs directory first

```bash
mkdir -p /home/mjlockboxsocial/vibepilot/logs
```

---

## Verify cron is set up

```bash
crontab -l
```

---

## To remove cron job later

```bash
crontab -e
# Delete the line
# Save and exit
```

---

## DELETE THIS FILE AFTER SETUP

This file is temporary. Once you've set up the cron job, delete it:

```bash
git rm TEMP_CRON_COMMANDS.md
git commit -m "Remove temporary cron setup file"
git push
```
