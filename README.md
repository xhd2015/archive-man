# Archive Manager
Library and command line tools used to manage archived photos, documents and other files.

# Installation
Install CLI:

```sh
go get github.com/xhd2015/archive-man/cmd/archive-man@latest
```

# Command line usage
```sh
# find all ._ hidden files
archive-man inspect --prefix ._ --count D:\iCloud-Photos\
archive-man inspect --prefix ._ --limit 10 D:\iCloud-Photos\

# delete them
archive-man delete-files --prefix ._ D:\iCloud-Photos\
```

NOTE: the CLI is still in development, it's not quite stable.


# Sync

When sync files, the modified time will be reserved.