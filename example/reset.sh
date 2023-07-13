#!/bin/bash
make -j9
for i in `seq 1 3`
do
   lxc exec c$i -- bash -c "kill -9 \$(pidof microd) \$(pidof microctl)"
   lxc file push /root/go/bin/microctl c$i/usr/bin/microctl
   lxc file push /root/go/bin/microd c$i/usr/bin/microd
   lxc exec c$i -- bash -c "rm -rf /micro /tmp/microlog"
   lxc exec c$i -- bash -ci "(microd --state-dir /micro >> /tmp/microlog 2>&1) & disown"
done

lxc exec c1 -- bash -c " microctl --state-dir /micro init c1 20.0.0.252:9777 --bootstrap "

if [[ $1 != "skip" ]]; then
  sleep 2
  token_node2=$(lxc exec c1 -- bash -c "microctl --state-dir /micro tokens add member2")
  token_node3=$(lxc exec c1 -- bash -c "microctl --state-dir /micro tokens add member3")
  lxc exec c2 -- bash -c "microctl --state-dir /micro init member2 20.0.0.138:9777 --token ${token_node2}"
  lxc exec c3 -- bash -c "microctl --state-dir /micro init member3 20.0.0.168:9777 --token ${token_node3}"
fi
echo Ready

