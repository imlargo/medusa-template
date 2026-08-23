package push

import (
	"encoding/json"

	"github.com/SherClockHolmes/webpush-go"
)

type PushNotificationSender interface {
	Send(subscription *Subscription, payload any) error
}

type pushNotificationSender struct {
	vapidPrivateKey string
	vapidPublicKey  string
	subscriber      string
}

func NewPushNotificationSender(vapidPrivateKey string, vapidPublicKey string, subscriber string) PushNotificationSender {
	return &pushNotificationSender{
		vapidPrivateKey: vapidPrivateKey,
		vapidPublicKey:  vapidPublicKey,
		subscriber:      subscriber,
	}
}

func (p *pushNotificationSender) Send(subscription *Subscription, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	webpushSub := &webpush.Subscription{
		Endpoint: subscription.Endpoint,
		Keys: webpush.Keys{
			P256dh: subscription.P256dh,
			Auth:   subscription.Auth,
		},
	}

	resp, err := webpush.SendNotification(payloadBytes, webpushSub, &webpush.Options{
		Subscriber:      p.subscriber,
		VAPIDPublicKey:  p.vapidPublicKey,
		VAPIDPrivateKey: p.vapidPrivateKey,
		TTL:             30,
	})
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	return nil
}
