package discordbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/pikachu0310/discord-codex-multimodal-bot/internal/alarm"
	"github.com/pikachu0310/discord-codex-multimodal-bot/internal/chat"
	"github.com/pikachu0310/discord-codex-multimodal-bot/internal/codex"
	"github.com/pikachu0310/discord-codex-multimodal-bot/internal/voice"
	"github.com/pikachu0310/discord-codex-multimodal-bot/internal/whisper"
)

// Bot wires event handlers and sub-systems (voice transcription, chat, etc).
type Bot struct {
	session *discordgo.Session
	voice   *voice.Manager
	chat    *chat.Manager
	alarm   *alarm.Client
}

const commandNameAlarm = "alarm"

// New creates a ready-to-run bot with voice transcription enabled.
func New(token, transcriptChannelID string, whisperClient *whisper.Client, store *codex.Store, namer *codex.ThreadNamer, codexClient codex.Client, alarmClient *alarm.Client) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	if whisperClient == nil {
		return nil, fmt.Errorf("whisper client is nil")
	}
	session.StateEnabled = true
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsMessageContent

	bot := &Bot{
		session: session,
		voice:   voice.NewManager(session, whisperClient, transcriptChannelID),
		chat:    chat.NewManager(session, store, namer, codexClient),
		alarm:   alarmClient,
	}

	session.AddHandler(bot.onReady)
	session.AddHandler(bot.handleMessageCreate)
	session.AddHandler(bot.handleInteraction)
	session.AddHandler(bot.handleThreadMessage)

	return bot, nil
}

// Run opens the Discord gateway and blocks until ctx is canceled.
func (b *Bot) Run(ctx context.Context) error {
	if err := b.session.Open(); err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}
	log.Println("bot is running")
	defer b.session.Close()

	<-ctx.Done()
	b.chat.Close()
	b.voice.Shutdown()
	return nil
}

func (b *Bot) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || m.GuildID == "" {
		return
	}
}

func (b *Bot) handleInteraction(s *discordgo.Session, ic *discordgo.InteractionCreate) {
	if ic == nil || ic.Type != discordgo.InteractionApplicationCommand {
		return
	}

	switch ic.ApplicationCommandData().Name {
	case "join":
		chID, err := b.findUserVoiceChannel(ic.GuildID, ic.Member.User.ID)
		if err != nil {
			b.respondEphemeral(ic, fmt.Sprintf("VC を特定できません: %v", err))
			return
		}
		if err := b.voice.Join(ic.GuildID, ic.Member.User.ID); err != nil {
			b.respondEphemeral(ic, fmt.Sprintf("参加に失敗しました: %v", err))
			return
		}
		b.respond(ic, fmt.Sprintf("参加しました (チャンネル: <#%s>)", chID))
	case "leave":
		if err := b.voice.Leave(ic.GuildID); err != nil {
			b.respondEphemeral(ic, fmt.Sprintf("退出に失敗しました: %v", err))
			return
		}
		b.respond(ic, "退出しました。")
	case commandNameAlarm:
		b.handleAlarmCommand(ic)
	default:
		b.chat.HandleInteraction(ic)
	}
}

func (b *Bot) handleAlarmCommand(ic *discordgo.InteractionCreate) {
	data := ic.ApplicationCommandData()
	var message string
	for _, opt := range data.Options {
		if opt.Name == "message" {
			message = strings.TrimSpace(opt.StringValue())
			break
		}
	}
	if message == "" {
		b.respondEphemeral(ic, "アラーム内容を入力してください。")
		return
	}
	if b.alarm == nil {
		b.respondEphemeral(ic, "/alarm は現在無効化されています。HOME_ASSISTANT_* の設定を確認してください。")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	payload := alarm.Payload{
		Message:     message,
		GuildID:     ic.GuildID,
		ChannelID:   ic.ChannelID,
		TriggeredAt: time.Now(),
	}

	if ch, err := b.session.State.Channel(ic.ChannelID); err == nil && ch != nil {
		payload.ChannelName = ch.Name
	} else if ch, err := b.session.Channel(ic.ChannelID); err == nil && ch != nil {
		payload.ChannelName = ch.Name
	}

	user := ic.User
	if user == nil && ic.Member != nil {
		user = ic.Member.User
	}
	if user != nil {
		payload.TriggeredByID = user.ID
		payload.TriggeredBy = user.Username
		if ic.Member != nil && ic.Member.Nick != "" {
			payload.TriggeredBy = ic.Member.Nick
		} else if user.GlobalName != "" {
			payload.TriggeredBy = user.GlobalName
		} else if user.Discriminator != "" && user.Discriminator != "0" {
			payload.TriggeredBy = fmt.Sprintf("%s#%s", user.Username, user.Discriminator)
		}
	}

	if err := b.alarm.Trigger(ctx, payload); err != nil {
		log.Printf("failed to trigger alarm: %v", err)
		b.respondEphemeral(ic, fmt.Sprintf("アラーム送信に失敗しました: %v", err))
		return
	}
	b.respondEphemeral(ic, "アラームを送信しました。")
}

func (b *Bot) handleThreadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}
	b.chat.HandleThreadMessage(m)
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	if err := b.chat.RegisterCommands(); err != nil {
		log.Printf("failed to register commands: %v", err)
	} else {
		log.Printf("slash commands registered (app_id=%s)", s.State.User.ID)
	}

	// Register voice commands join/leave
	appID := s.State.User.ID
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "join",
			Description: "あなたがいる VC へ参加します",
		},
		{
			Name:        "leave",
			Description: "VC から退出します",
		},
		{
			Name:        commandNameAlarm,
			Description: "スマホにアラームを送信します",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "message",
					Description: "アラーム内容（例: 10 分後に休憩）",
					Required:    true,
				},
			},
		},
	}
	for _, cmd := range commands {
		if _, err := s.ApplicationCommandCreate(appID, "", cmd); err != nil {
			log.Printf("failed to register command %s: %v", cmd.Name, err)
		}
	}
}

func (b *Bot) findUserVoiceChannel(guildID, userID string) (string, error) {
	guild, err := b.session.State.Guild(guildID)
	if err != nil {
		guild, err = b.session.Guild(guildID)
		if err != nil {
			return "", err
		}
	}
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID {
			return vs.ChannelID, nil
		}
	}
	return "", fmt.Errorf("ユーザーは VC に接続していません")
}

func (b *Bot) respond(ic *discordgo.InteractionCreate, content string) {
	err := b.session.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
	if err != nil && !strings.Contains(err.Error(), "Interaction has already been acknowledged") {
		log.Printf("respond failed: %v", err)
	}
}

func (b *Bot) respondEphemeral(ic *discordgo.InteractionCreate, content string) {
	err := b.session.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil && !strings.Contains(err.Error(), "Interaction has already been acknowledged") {
		log.Printf("respond failed: %v", err)
	}
}
