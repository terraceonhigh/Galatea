/*
 * fuse_lowlevel.h — minimal clean-room stub for Galatea's libfuse shim.
 *
 * The real libfuse fuse_lowlevel.h declares the full low-level (request/reply)
 * API. Galatea's shim implements the high-level operation model over an NFS
 * server, not the low-level channel protocol, so this header carries only what
 * tools actually reference from the served paths: the fuse_chan type (via
 * fuse_common.h) and fuse_chan_fd. Written from observed need, not transcribed
 * from libfuse — declarations are our own, added as required (clean-room), so
 * the shipped tree stays free of copyleft we don't own.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */
#ifndef GALATEA_FUSE_LOWLEVEL_H_
#define GALATEA_FUSE_LOWLEVEL_H_

#include "fuse_common.h"

#ifdef __cplusplus
extern "C" {
#endif

/*
 * Return the file descriptor of a fuse_chan. Galatea has no kernel FUSE channel
 * (it serves over an NFS loopback socket), so the shim returns -1; tools use
 * this only for diagnostics/select on platforms with a real channel.
 */
int fuse_chan_fd(struct fuse_chan *ch);

#ifdef __cplusplus
}
#endif

#endif /* GALATEA_FUSE_LOWLEVEL_H_ */
