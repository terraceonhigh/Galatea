/* Minimal fuse_misc.h for the Galatea libfuse shim.
 *
 * Upstream libfuse's lib/fuse_misc.h #includes the autoconf-generated config.h
 * and defines mutex/stat-nsec helpers used across lib/. The only thing the one
 * file we vendor (fuse_opt.c) needs from it is FUSE_SYMVER, which is a no-op on
 * macOS (Mach-O has no versioned symbols). So this stands in for the real header
 * without dragging the autoconf build in. */
#ifndef GALATEA_FUSE_MISC_H
#define GALATEA_FUSE_MISC_H

#define FUSE_SYMVER(x)

#endif
