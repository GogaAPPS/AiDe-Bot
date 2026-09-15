package maxapi

import "github.com/max-messenger/max-bot-api-client-go/v2/model"

const (
	CallbackMainMenu      = "main_menu"
	CallbackBack          = "back"
	CallbackNewChat       = "new_chat"
	CallbackDesignDevelop = "design_develop"
)

const menuButtonText = "Меню"
const backButtonText = "Назад"

func newMenuKeyboard() *model.Keyboard {
	keyboard := model.NewKeyboard()
	keyboard.AddRow().AddCallback(menuButtonText, model.IntentDefault, CallbackMainMenu)

	return keyboard
}

func newMainMenuKeyboard() *model.Keyboard {
	keyboard := model.NewKeyboard()
	keyboard.
		AddRow().
		AddCallback("Новый чат", model.IntentDefault, CallbackNewChat)
	keyboard.
		AddRow().
		AddCallback("Генерация дизайна(Develop)", model.IntentDefault, CallbackDesignDevelop)
	keyboard.
		AddRow().
		AddCallback(backButtonText, model.IntentDefault, CallbackBack)

	return keyboard
}
