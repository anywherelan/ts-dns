// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dns

import (
	"time"
)

// We use file-ignore below instead of ignore because on some platforms,
// the lint exception is necessary and on others it is not,
// and plain ignore complains if the exception is unnecessary.

//lint:file-ignore U1000 reconfigTimeout is used on some platforms but not others

// reconfigTimeout is the time interval within which Manager.{Up,Down} should complete.
//
// This is particularly useful because certain conditions can cause indefinite hangs
// (such as improper dbus auth followed by contextless dbus.Object.Call).
// Such operations should be wrapped in a timeout context.
const reconfigTimeout = time.Second
