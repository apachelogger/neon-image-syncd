A daemon to poll rsync and sync neon images.

## Why

With the advance of openqa we have a need to get access to ISOs for
quality contorl. This is easiest done by only getting ISOs from our own mirror
where bandwith restrictions and so forth are no concern. The daemon at hand
is a simple semaphore around rsync such that we can securely invoke it from a
remote (jenkins) as well as a systemd timer by serializing the sync requests
in our daemon.

We may also operate this as a public mirror depending on load, or ona separate
machine. To always have a mirror immediately available with decent speed.

## How

For security purposes this is split in effectively three different pieces.

- neon-image-syncd is the daemon part written in Go, it provides a REST API
  with a single endpoint to command a sync. This endpoint is blocking until
  the request gets run (i.e. the request is effectively queued until
  a previous request finished).
- client.rb is a client implement written in ruby to command a sync from the CLI
  but still going through the request queue.
- The webserver serving the content (that's regular apache/nginx).

To facilitate this three way split groups and users are separated.

- neon-image-sync has a user and group. It talks to the server over a socket
  to which the neon-image-sync group has write access. It can not edit the
  images directly!
- apache/ngix have www-data as group. The resynced images are readable by
  www-data BUT not writable in any form or fashion. Neither can the servers
  talk to the daemon socket.
- neon-image-syncd has a user and group. It is also group member of
  neon-image-sync. The daemon socket is writable by neon-image-sync
  and the daemon chowns image data such that www-data can READ them.

Effectively this means that only the daemon may write images. By extension
the daemon and client are respectively run in their own users when run by
systemd.

Talking about systemd, the client has a sync timer as a safety net to prevent
going out of date. The regular way of syncing is for the ISO builders to SSH
into neon-image-sync and run the client, which blockingly queues a sync to be
run as soon as the queue is empty.
Ideally once the ISO job is finished this also means the mirror is up to date
(unless there were problems with syncing of course, which the client will
reflect and potentially fail the ISO build job on).
