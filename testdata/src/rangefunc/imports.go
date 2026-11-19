package rangefunc

import (
	_ "iter"  // want `import of iter is forbidden`
	it "iter" // want `import of iter is forbidden`
	_ "rangefunc/iter"
)

var _ it.Seq[int]
