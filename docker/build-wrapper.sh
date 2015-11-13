set -e

ls -lnd . | (read mod links uid gid rest; echo "amberbuild:x:$uid:$gid::/go:/bin/sh") >>/etc/passwd
echo "amberbuild:*:::::::" >>/etc/shadow
su -p -c "PATH=\"$PATH\" ; GO15VENDOREXPERIMENT=1 ; export PATH GO15VENDOREXPERIMENT ; $1" amberbuild
