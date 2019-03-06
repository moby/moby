package notifier

// ObjectChangeNotifier is an interface defining an object that can be called
// back to if the Reconciler decides that it needs to take another pass at some
// object. The ObjectChangeNotifier may seem a bit excessive, but it provides
// the key functionality of decoupling the synchronous part of the Reconciler
// from the asynchronous part of the component that calls into it. Without it,
// the Reconciler might have both synchronous and asynchronous components in
// the same object (a pattern common in Swarmkit), which would make testing
// much more difficult.
type ObjectChangeNotifier interface {
	// Notify indicates the kind and ID of an object that should be reconciled
	Notify(kind, id string)
}

// Register is an interface defining an object that an ObjectChangeNotifier can
// register itself with
type Register interface {
	Register(ObjectChangeNotifier)
}

// NotificationForwarder is a type used to to forward notifications called on
// itself to another ObjectChangeNotifier. The purpose of this is to break the
// circular dependency between the Reconciler and the Manager. To this end,
// we create NotificationForwarder, which we pass to the Reconciler. Then, we
// create a Reconciler, to which we pass the NotificationForwarder. Finally, we
// create a Manager, which implements ObjectChangeNotifier, and set it as the
// Receiver of of the NotificationForwarder.
//
// This way, instead of looking like this, with a circular dependency:
//
//  ____________                  _________
// |            |   depends on   |         |
// | Reconciler | <------------> | Manager |
// |____________|                |_________|
//
// It looks like this, with no circular dependency:
//
//  ____________                _______________________
// |            |  depends on  |                       |
// | Reconciler | -----------> | NotificationForwarder |
// |____________|              |_______________________|
//     /|\                          /|\
//      | depends on                 |
//  ____|____                        |
// |         |      depends on       |
// | Manager | ----------------------
// |_________|
//
// More complicated, by prevents us from having to initialize the Reconciler
// inside of the Manager. The code to use this ends up look like this:
//
// ```
// nf := NewNotificationForwarder()
// r := NewReconciler(nf, cli)
// m := NewManager(r, nf)
// ```
type NotificationForwarder interface {
	ObjectChangeNotifier
	Register
}

type notificationForwarder struct {
	notifier ObjectChangeNotifier
}

// NewNotificationForwarder returns a NotificationForwarder that passes
// notifications to the registered receiver.
func NewNotificationForwarder() NotificationForwarder {
	return &notificationForwarder{}
}

// Notify notifies the registered receiver, if one exists.
func (n *notificationForwarder) Notify(kind, id string) {
	if n.notifier != nil {
		n.notifier.Notify(kind, id)
	}
}

// Register associates an ObjectChangeNotifier with this forwarder
func (n *notificationForwarder) Register(notifier ObjectChangeNotifier) {
	n.notifier = notifier
}
