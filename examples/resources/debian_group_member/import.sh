# The import ID for group_member is "username:groupname":
terraform import debian_group_member.alice_sudo 'user=root;host=dev01.example.com;port=22;id=alice:sudo'

# User defaults to "root" when omitted:
terraform import debian_group_member.alice_sudo 'host=dev01.example.com;port=22;id=alice:sudo'

# Look up a public key in the provider's private_keys map:
terraform import debian_group_member.alice_sudo 'user=root;host=dev01.example.com;port=22;public_key=ssh-ed25519 AAAA...;id=alice:sudo'
