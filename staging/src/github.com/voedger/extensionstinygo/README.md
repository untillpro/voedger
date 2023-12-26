# extensions-tinygo

## Usage

```go
package main

import (
	ext "github.com/heeus/extensions-tinygo"
)

//export exampleExtension
func exampleExtension() {
	event := ext.GetValue(ext.KeyBuilder(ext.StorageEvent, ext.NullEntity))

	if event.AsString("qname") == "air.UpdateSubscription" {
		json := event.AsValue("arg")
		subscr := json.AsValue("subscription")
		customer := json.AsValue("customer")
		mail := ext.NewValue(ext.KeyBuilder(ext.StorageSendmail, ext.NullEntity))
		mail.PutString("from", "test@gmail.com")
		mail.PutString("to", customer.AsString("email"))
		mail.PutString("body", "Your subscription has been updated. New status: "+subscr.AsString("status"))
	}
}
```