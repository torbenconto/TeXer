

### Why a polling watcher instead of using kqueue or inotify?
Because great implementations for kernel event catchers already exist in golang with fsnotify. Also, kqueue and things like it are pretty heavy on system resources and I would like TeXer to be as lightweight as possible.

Now for the real reason: They suck to work with! I have to build implementations for every notification framework (and thats a lot of work!), it also wouldn't work with anything but a physical filesystem (incompatible with things like FUSE).