package nnet

import "github.com/nknorg/nnet/util"

// ApplyMiddleware add a middleware to overlay or local node
func (nn *NNet) ApplyMiddleware(f interface{}) error {
	applied := false
	errs := util.NewErrors()

	err := nn.GetLocalNode().ApplyMiddleware(f)
	if err == nil {
		applied = true
	} else {
		errs = append(errs, err)
	}

	err = nn.Network.ApplyMiddleware(f)
	if err == nil {
		applied = true
	} else {
		errs = append(errs, err)
	}

	for _, router := range nn.GetRouters() {
		err = router.ApplyMiddleware(f)
		if err == nil {
			applied = true
		} else {
			errs = append(errs, err)
		}
	}

	if !applied {
		return errs.Merged()
	}

	return nil
}

// MustApplyMiddleware is the same as ApplyMiddleware, but will panic if an
// error occurs
func (nn *NNet) MustApplyMiddleware(f interface{}) {
	err := nn.ApplyMiddleware(f)
	if err != nil {
		panic(err)
	}
}
