package resilience

import "fmt"

// BusinessError represents a non-retriable domain error that should trigger a BPMN error catch event
type BusinessError struct {
	Code    string
	Message string
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("BPMN Business Error [%s]: %s", e.Code, e.Message)
}

// NewBusinessError creates a new BPMN business error
func NewBusinessError(code, message string) *BusinessError {
	return &BusinessError{
		Code:    code,
		Message: message,
	}
}

// RetriableError represents a transient error that should cause Zeebe to retry the job
type RetriableError struct {
	Err     error
	Message string
}

func (e *RetriableError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Retriable Error: %s (caused by: %v)", e.Message, e.Err)
	}
	return fmt.Sprintf("Retriable Error: %s", e.Message)
}

func (e *RetriableError) Unwrap() error {
	return e.Err
}

// NewRetriableError creates a new retriable error
func NewRetriableError(message string, cause error) *RetriableError {
	return &RetriableError{
		Message: message,
		Err:     cause,
	}
}

// Common BPMN Error Codes
const (
	ErrPaymentDeclined  = "ERR_PAYMENT_DECLINED"
	ErrInvalidOrder     = "ERR_INVALID_ORDER"
	ErrOutOfStock       = "ERR_OUT_OF_STOCK"
	ErrCustomerNotFound = "ERR_CUSTOMER_NOT_FOUND"
)
