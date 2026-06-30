package hw03frequencyanalysis

import (
	"testing"

	"github.com/stretchr/testify/require" //nolint:depguard
)

var text = `Как видите, он  спускается  по  лестнице  вслед  за  своим
	другом   Кристофером   Робином,   головой   вниз,  пересчитывая
	ступеньки собственным затылком:  бум-бум-бум.  Другого  способа
	сходить  с  лестницы  он  пока  не  знает.  Иногда ему, правда,
		кажется, что можно бы найти какой-то другой способ, если бы  он
	только   мог   на  минутку  перестать  бумкать  и  как  следует
	сосредоточиться. Но увы - сосредоточиться-то ему и некогда.
		Как бы то ни было, вот он уже спустился  и  готов  с  вами
	познакомиться.
	- Винни-Пух. Очень приятно!
		Вас,  вероятно,  удивляет, почему его так странно зовут, а
	если вы знаете английский, то вы удивитесь еще больше.
		Это необыкновенное имя подарил ему Кристофер  Робин.  Надо
	вам  сказать,  что  когда-то Кристофер Робин был знаком с одним
	лебедем на пруду, которого он звал Пухом. Для лебедя  это  было
	очень   подходящее  имя,  потому  что  если  ты  зовешь  лебедя
	громко: "Пу-ух! Пу-ух!"- а он  не  откликается,  то  ты  всегда
	можешь  сделать вид, что ты просто понарошку стрелял; а если ты
	звал его тихо, то все подумают, что ты  просто  подул  себе  на
	нос.  Лебедь  потом  куда-то делся, а имя осталось, и Кристофер
	Робин решил отдать его своему медвежонку, чтобы оно не  пропало
	зря.
		А  Винни - так звали самую лучшую, самую добрую медведицу
	в  зоологическом  саду,  которую  очень-очень  любил  Кристофер
	Робин.  А  она  очень-очень  любила  его. Ее ли назвали Винни в
	честь Пуха, или Пуха назвали в ее честь - теперь уже никто  не
	знает,  даже папа Кристофера Робина. Когда-то он знал, а теперь
	забыл.
		Словом, теперь мишку зовут Винни-Пух, и вы знаете почему.
		Иногда Винни-Пух любит вечерком во что-нибудь поиграть,  а
	иногда,  особенно  когда  папа  дома,  он больше любит тихонько
	посидеть у огня и послушать какую-нибудь интересную сказку.
		В этот вечер...`

func TestTop10(t *testing.T) {
	t.Run("no words in empty string", func(t *testing.T) {
		require.Len(t, Top10(""), 0)
	})

	t.Run("positive test with additional task rules", func(t *testing.T) {
		expected := []string{
			"а",         // 8
			"он",        // 8
			"и",         // 6
			"ты",        // 5
			"что",       // 5
			"в",         // 4
			"его",       // 4
			"если",      // 4
			"кристофер", // 4
			"не",        // 4
		}
		require.Equal(t, expected, Top10(text))
	})

	t.Run("normalizes case and trims punctuation only at word edges", func(t *testing.T) {
		input := "Нога нога! нога, 'нога' какой-то какойто dog,cat dog...cat dogcat"
		expected := []string{
			"нога",      // 4
			"dog,cat",   // 1
			"dog...cat", // 1
			"dogcat",    // 1
			"какой-то",  // 1
			"какойто",   // 1
		}
		require.Equal(t, expected, Top10(input))
	})

	t.Run("single hyphen is not a word but multiple hyphens are preserved", func(t *testing.T) {
		input := "- ------- ------- слово словa"
		expected := []string{
			"-------", // 2
			"словa",   // 1
			"слово",   // 1
		}
		require.Equal(t, expected, Top10(input))
	})

	t.Run("emoji is preserved as a word symbol", func(t *testing.T) {
		input := "🙂🙂 🙂 🙂! hello🙂 hello🙂. 🙂hello🙂"
		expected := []string{
			"hello🙂",  // 2
			"🙂",       // 2
			"🙂hello🙂", // 1
			"🙂🙂",      // 1
		}
		require.Equal(t, expected, Top10(input))
	})

	t.Run("single and repeated punctuation are words", func(t *testing.T) {
		input := "! ... ! ... слово"
		expected := []string{
			"!",     // 2
			"...",   // 2
			"слово", // 1
		}
		require.Equal(t, expected, Top10(input))
	})

	t.Run("splits words on newline tab and other whitespace", func(t *testing.T) {
		input := "alpha\n beta\talpha\r\ngamma\vbeta\ffinal"
		expected := []string{
			"alpha", // 2
			"beta",  // 2
			"final", // 1
			"gamma", // 1
		}
		require.Equal(t, expected, Top10(input))
	})
}
