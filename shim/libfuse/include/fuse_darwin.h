/*
 * fuse_darwin.h — minimal clean-room stub for Galatea's libfuse shim.
 *
 * macFUSE ships a fuse_darwin.h carrying Darwin-only extras (volume naming,
 * daemon helpers, fuse_darwin_* calls). Stock libfuse and Galatea's shim do not.
 * In the libfuse-2.x line, tools (sshfs et al.) `#include <fuse_darwin.h>` under
 * __APPLE__ but, in the operation paths Galatea serves, do not call its
 * functions. This header exists solely to satisfy that include.
 *
 * Written from the observed need (an empty include), NOT transcribed from
 * macFUSE's header — declarations are added only if a real tool is found to call
 * them, and would be our own ABI-compatible declarations (clean-room), keeping
 * the shipped tree free of copyleft we don't own.
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */
#ifndef GALATEA_FUSE_DARWIN_H_
#define GALATEA_FUSE_DARWIN_H_

/* Intentionally minimal. fuse.h / fuse_common.h provide everything the served
 * paths need; add Darwin-specific declarations here only when a tool requires
 * them, and write them clean-room. */

#endif /* GALATEA_FUSE_DARWIN_H_ */
