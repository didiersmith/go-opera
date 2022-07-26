set -e
# rsync -hvrPL --exclude '*.g' * ubuntu@dexter-de3:~/go-opera/
# exit 0
servers=(
    dexter-uk3
    dexter-de3
    dexter-nj3
    # dexter-ca3
)
for server in "${servers[@]}"; do
    echo "Syncing to $server"
    rsync -hvrPL -e "ssh -i ~/.ssh/id_arnogmail" --exclude '*.g' build/* ubuntu@$server:~/go-opera/build/
    echo "Finished syncing to $server"
done

#    dexter-de - retired
