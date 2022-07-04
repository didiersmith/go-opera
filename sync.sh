set -e
# rsync -hvrPL --exclude '*.g' * ubuntu@dexter-de3:~/go-opera/
# exit 0
servers=(
    # dexter-uk3
    dexter-de3
    dexter-nj3
    dexter-ca3
    dexter-va
    # dexter2
    # dexter3
    # dexter1
)
for server in "${servers[@]}"; do
    echo "Syncing to $server"
    rsync -hvrPL --exclude '*.g' build/* ubuntu@$server:~/go-opera/build/
    echo "Finished syncing to $server"
done

#    dexter-de - retired
