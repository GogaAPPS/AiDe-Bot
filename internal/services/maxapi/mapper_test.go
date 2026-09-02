package maxapi

import (
	"testing"

	"github.com/max-messenger/max-bot-api-client-go/v2/model"
)

func TestCallbackEventFromUpdate(t *testing.T) {
	update := model.Update{
		UpdateType: model.UpdateMessageCallback,
		ChatID:     42,
		UserID:     7,
		Callback: &model.Callback{
			CallbackID: "callback-1",
			Payload:    "  " + CallbackNewChat + "  ",
		},
	}

	callback, ok := CallbackEventFromUpdate(update)
	if !ok {
		t.Fatal("expected callback event")
	}
	if callback.ID != "callback-1" || callback.Payload != CallbackNewChat {
		t.Fatalf("unexpected callback: %+v", callback)
	}
	if callback.Target.ChatID != 42 || callback.Target.UserID != 7 {
		t.Fatalf("unexpected target: %+v", callback.Target)
	}
}

func TestCallbackEventFromUpdateRejectsOtherUpdates(t *testing.T) {
	_, ok := CallbackEventFromUpdate(model.Update{UpdateType: model.UpdateMessageCreated})
	if ok {
		t.Fatal("message update should not be treated as callback")
	}
}
