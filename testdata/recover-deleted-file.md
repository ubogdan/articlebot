---
title: "How to recover a deleted file when a process keeps it open"
date: 2021-07-22
description: "We all make small mistakes every day. Unfortunately, some of these mistakes are unforgivable."
categories:
  - Linux
tags:
  - linux
  - recovery
---

We all make small mistakes every day. Unfortunately, some of these mistakes are unforgivable.

First, we need to identify the PID of the running process by using the ps command

```bash
# ps -ax | grep webapp
45783 ?        Ssl    0:09 webapp
```

After that we need to check if the file is still in use.

```bash
# ls -al /proc/`pidof webapp`/fd
lrwx------ 1 webuser webuser 64 Jul 22 21:23 7 -> /opt/webapp/var/database.sqlite (deleted)
```

To recover the file, we need the inode where the file was stored, which can be retrieved using the lsof command.

```bash
# lsof -p `pidof webapp` | grep database.sqlite
webapp 49057 webuser    7u      REG                8,3   237568  3674442 /opt/webapp/var/database.sqlite
```

Check out this link for more info: [More info](https://example.com/recovery)

And here is another link: [Support](#)
