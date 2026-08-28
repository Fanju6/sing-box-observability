#!/system/bin/sh

MODDIR=${0%/*}
CTL="$MODDIR/bin/sing-box-observabilityctl"

echo "重启 sing-box Observability..."
"$CTL" restart
echo "Web 面板：http://127.0.0.1:9095"
