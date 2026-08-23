#!/system/bin/sh

umask 077

# Optional Magisk service.d entry. Install only after manual startup succeeds.
until [ "$(getprop sys.boot_completed)" = "1" ]; do
  sleep 2
done

/data/adb/sing-box-observability/sing-box-observabilityctl start
