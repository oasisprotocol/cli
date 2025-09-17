package common

import "fmt"

// RentRefundWarning is a standardized message shown before renting, topping up, or canceling a machine.
const RentRefundWarning = "WARNING: Machine rental is non-refundable. You will not get a refund for the already paid term if you cancel."

// PrintRentRefundWarning prints the standardized, user-facing refund policy warning.
func PrintRentRefundWarning() {
	fmt.Println(RentRefundWarning)
}
