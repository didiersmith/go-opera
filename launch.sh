#!/bin/bash
accounts=$(ls ~/keyfiles/0x* | xargs -n 1 basename | tr '\n' ',' | sed 's/,$//')
#accounts=$(ls ~/keyfiles/0x* | head -50 | xargs -n 1 basename | tr '\n' ',' | sed 's/,$//')

./opera --genesis mainnet.g \
    --http \
    --http.addr="127.0.0.1" \
    --http.vhosts="*" \
    --http.corsdomain="*" \
    --http.api="ftm,eth,debug,admin,web3,personal,net,txpool,sfc,dag" \
    --ws \
    --ws.addr="127.0.0.1" \
    --ws.origins="*" \
    --ws.api="ftm,eth,debug,admin,web3,personal,net,txpool,sfc,dag" \
    --verbosity=3 \
    --maxpeers=1000 \
    --maxpendpeers=500 \
    --txpool.globalqueue=16384 \
    --txpool.accountqueue=256 \
    --txpool.accountslots=32 \
    --txpool.globalslots=16384 \
    --cache=8192 \
    --allow-insecure-unlock \
    --password=/home/ubuntu/keyfiles/passes \
    --unlock=$accounts \
    --txpool.locals=$accounts

