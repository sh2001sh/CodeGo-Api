package app

import "errors"

var (
	ErrSetupAlreadyCompleted = errors.New("系统已经初始化完成")
	ErrSetupUsernameTooLong  = errors.New("用户名长度不能超过12个字符")
	ErrSetupPasswordMismatch = errors.New("两次输入的密码不一致")
	ErrSetupPasswordTooShort = errors.New("密码长度至少为8个字符")
)

type setupOperationError struct {
	message string
	err     error
}

func (e *setupOperationError) Error() string {
	if e == nil {
		return ""
	}
	return e.message + e.err.Error()
}

func (e *setupOperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func wrapSetupError(prefix string, err error) error {
	return &setupOperationError{
		message: prefix,
		err:     err,
	}
}
