package config

import "sync"

// Listeners for changes to the app status, like the loading page
var (
	statusMu   sync.Mutex
	statusSubs = map[chan struct{}]struct{}{}
)

// SubscribeStatus returns a channel that receives a value when the app status changes.
// Changes made while the last one is being handled are combined into one.
func SubscribeStatus() chan struct{} {
	ch := make(chan struct{}, 1)
	statusMu.Lock()
	statusSubs[ch] = struct{}{}
	statusMu.Unlock()
	return ch
}

func UnsubscribeStatus(ch chan struct{}) {
	statusMu.Lock()
	delete(statusSubs, ch)
	statusMu.Unlock()
}

// NotifyStatusChanged tells the listeners the app status changed
func NotifyStatusChanged() {
	statusMu.Lock()
	defer statusMu.Unlock()
	for ch := range statusSubs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// SetAppLoadingStep sets what aura is doing while it starts, and tells the listeners
func SetAppLoadingStep(step string) {
	AppLoadingStep = step
	NotifyStatusChanged()
}
