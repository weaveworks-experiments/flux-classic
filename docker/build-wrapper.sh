set -e

ls -lnd . | (read mod links uid gid rest; echo "amberbuild:x:$uid:$gid::/go:/bin/sh") >>/etc/passwd
echo "amberbuild:*:::::::" >>/etc/shadow
su -p -c "export PATH=\"$PATH\" ; $1" amberbuild
