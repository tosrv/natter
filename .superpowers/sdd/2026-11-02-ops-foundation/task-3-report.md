# Task 3 report
Status: DONE_WITH_CONCERNS
Files: docker-compose.yml, docker-compose.dev.yml, deploy/nginx.conf.example, .env.example, README.md ("Operasional" section, Indonesian).
Watchtower pin: nickfedor/watchtower:1.23.2 (GitHub API blocked 403; verified via Docker Hub tags API: latest == 1.23.2, pushed 2026-10-22).
Verified:
- `docker compose config` OK (with temp .env POSTGRES_PASSWORD=test); `grep watchtower`: enable=true + pre-update only on app, enable=false on backup, none on postgres.
- dev overlay `config` OK (app + backup build: . + extra_ca secret).
- Ran postgres:16 + deploy/ mounted with the compose PG* env: backup.sh preupdate -> file pmess-...-preupdate.dump created; backup-cron.sh printed "backup scheduled" next 02:00 UTC.
Not run: image build / full stack up / app healthcheck / watchtower hooks / restore.sh (final image unbuildable in sandbox).
Concerns: backup service is not auto-updated by Watchtower (documented manual pull); healthcheck uses curl (needs curl in image - Task 2 image must include it); Docker Hub 1.23.2 verified, GitHub release notes not.
Cleanup: temp containers/network/.env removed.

## Fix round 1
- dev overlay: watchtower `profiles: ["prod-only"]`, app/backup `image: pmess:dev`.
- PGPASSWORD entries now use `:?` guard; README rollback uses `up -d app backup`.
- Verified: dev `config --services` lists postgres, app, backup only; images pmess:dev; prod config still valid.
