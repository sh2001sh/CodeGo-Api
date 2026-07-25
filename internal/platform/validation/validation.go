package validation

import "github.com/go-playground/validator/v10"

// Validate is the shared validator instance for runtime input validation.
var Validate = validator.New()
