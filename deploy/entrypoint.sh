#!/bin/sh
set -eu

# Emit a one-line capability banner so operators can see what the routing
# agent will be allowed to do. Non-fatal — d2ip self-checks again at startup.
if [ -x /usr/sbin/capsh ]; then
    /usr/sbin/capsh --print 2>/dev/null | sed -n 's/^Current: //p' | head -n1 || true
fi

exec "$@"
