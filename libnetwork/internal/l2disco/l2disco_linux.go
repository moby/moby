// Package l2disco sends ARP and Neighbour Advertisement messages.
package l2disco

// L2Disco should be instantiated for each network namespace that needs to send
// ARP and/or NA messages for new interfaces.
//
// To send ARP messages, call [L2Disco.NewUnsolARP] to instantiate an [UnsolARP]
// object in the network namespace that needs to send the ARP messages. To send
// a message, call [UnsolARP.Send] as many times as necessary, from any network
// namespace. Call [UnsolARP.Close] to close the ARP sender.
//
// Similarly, to send NA messages, use [L2Disco.NewUnsolNA] to instantiate an
// [UnsolNA] then use its [UnsolNA.Send] and [UnsolNA.Close] methods.
type L2Disco struct {
	arpData arpData
	naData  naData
}
