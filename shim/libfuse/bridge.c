/* C bridge for the libfuse shim. Defined here (not in the cgo preamble) so we
 * can include cgo's generated _cgo_export.h to reference the exported Go body
 * galateaFuseMain with cgo's exact signature — re-declaring it in the preamble
 * would conflict. (The readdir filler is pure C in the preamble, deliberately:
 * the app may itself be a Go program, and a Go-export callback would re-enter
 * our Go runtime from within the app's.) */
/* FUSE_USE_VERSION and _FILE_OFFSET_BITS come from the package #cgo CFLAGS. */
#include "fuse.h"
#include "_cgo_export.h"

/* fuse_main_real is the symbol behind fuse_main(...). We define it here with
 * fuse.h's exact signature (so an app links cleanly) and forward to the Go body
 * galateaFuseMain — exporting a Go function named fuse_main_real would conflict
 * with fuse.h's declaration of the same symbol. op_size is accepted for ABI
 * compatibility and ignored; user_data becomes fuse_get_context()->private_data. */
extern void galatea_set_user_data(void *ud); /* fuse_compat.c */
int fuse_main_real(int argc, char *argv[], const struct fuse_operations *op,
                   size_t op_size, void *user_data) {
	(void)op_size;
	galatea_set_user_data(user_data);
	return galateaFuseMain(argc, argv, (void *)op);
}
