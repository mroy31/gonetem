#!/bin/sh

if ! which dlv-dap ; then
	PATH="${GOPATH}/bin:$PATH"
fi
if [ "$DEBUG_AS_ROOT" = "true" ]; then
	DLV=$(which dlv-dap)
	exec sudo -E -A "$DLV" --only-same-user=false "$@"
else
	exec dlv-dap "$@"
fi
