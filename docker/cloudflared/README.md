Use Cloudflare Tunnel in dashboard-managed mode for `sudoku.endfieldhq.com`.

Recommended setup:

1. In Cloudflare Zero Trust, create a tunnel.
2. Add a public hostname:
   - Hostname: `sudoku.endfieldhq.com`
   - Service: `http://nginx:80`
3. Put the generated tunnel token into `docker/.env` as `CLOUDFLARE_TUNNEL_TOKEN=...`
4. Start the tunnel profile:

```bash
cd /Users/ila/dev/pet/sudoku/docker
docker compose --profile tunnel up -d
```

If you prefer a locally managed tunnel with credentials JSON, keep this directory and add your own `config.yml` and credentials mount.
