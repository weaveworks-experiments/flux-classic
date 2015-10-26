set -e

UIDGID=$(ls -lnd . | (read mod links uid gid rest; echo $uid:$gid))

GO=$(which go)
echo "build:x:$UIDGID::/go:/bin/sh" >>/etc/passwd
echo "build:*:::::::" >>/etc/shadow
su -p -c "$GO get . && $GO build ." build
