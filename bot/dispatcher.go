package bot

import (
	"context"
	"fmt"
	"go-stats/database"
	"go-stats/keymutex"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type handler = func(context.Context, Entities, tg.UpdateClass) error

type UpdateDispatcher struct {
	handlers map[uint32]handler
	botId    int64
	botApp   *string
	db       *gorm.DB
	api      *tg.Client
	clickCh  chan *database.Event
	logger   *zap.Logger
	keymutex *keymutex.KeyMutex
}

func NewUpdateDispatcher(botId int64, botApp *string, db *gorm.DB, clickCh chan *database.Event, logger *zap.Logger) UpdateDispatcher {
	return UpdateDispatcher{
		handlers: map[uint32]handler{},
		botId:    botId,
		botApp:   botApp,
		db:       db,
		api:      nil,
		clickCh:  clickCh,
		logger:   logger,
		keymutex: keymutex.New(47),
	}
}

type Entities struct {
	Short    bool
	Users    map[int64]*tg.User
	Chats    map[int64]*tg.Chat
	Channels map[int64]*tg.Channel
}

func (u *Entities) short() {
	u.Short = true
	u.Users = make(map[int64]*tg.User, 0)
	u.Chats = make(map[int64]*tg.Chat, 0)
	u.Channels = make(map[int64]*tg.Channel, 0)
}

func (u *UpdateDispatcher) addApi(api *tg.Client) {
	u.api = api
}

// Handle implements UpdateDispatcher.
func (u UpdateDispatcher) Handle(ctx context.Context, updates tg.UpdatesClass) error {
	// fmt.Println()
	// fmt.Println("Handle", updates.TypeName())
	var (
		e    Entities
		upds []tg.UpdateClass
	)
	switch u := updates.(type) {
	case *tg.Updates:
		upds = u.Updates
		e.Users = u.MapUsers().NotEmptyToMap()
		chats := u.MapChats()
		e.Chats = chats.ChatToMap()
		e.Channels = chats.ChannelToMap()
	case *tg.UpdatesCombined:
		upds = u.Updates
		e.Users = u.MapUsers().NotEmptyToMap()
		chats := u.MapChats()
		e.Chats = chats.ChatToMap()
		e.Channels = chats.ChannelToMap()
	case *tg.UpdateShort:
		upds = []tg.UpdateClass{u.Update}
		e.short()
	default:
		// *UpdateShortMessage
		// *UpdateShortChatMessage
		// *UpdateShortSentMessage
		// *UpdatesTooLong
		return nil
	}

	var err error
	for _, update := range upds {
		multierr.AppendInto(&err, u.dispatchSync(ctx, e, update))
	}
	return err
}

func (u *UpdateDispatcher) dispatch(ctx context.Context, e Entities, update tg.UpdateClass) error {
	go func() {
		err := u.dispatchSync(ctx, e, update)
		if err != nil {
			u.logger.Error("Error dispatching update", zap.Error(err))
		}
	}()
	return nil
}

func (u *UpdateDispatcher) dispatchSync(ctx context.Context, e Entities, update tg.UpdateClass) error {
	if update == nil {
		return nil
	}
	// fmt.Println(update)
	// Handle updates here, e.g., print the update
	event := database.Event{
		App:                *u.botApp,
		BotID:              u.botId,
		EventType:          "raw",
		EventSubtype:       strings.Replace(update.TypeName(), "update", "", 1),
		FromBot:            false,
		Data:               []string{},
		DataLowCardinality: []string{},
		DataInt:            []int64{},
		DataFlags:          []bool{},
		ChatID:             0,
		ChatType:           "",
		UserID:             0,
		SessionID:          int16(0),
		ContentID:          "",
		Language:           "",
		UserCreatedAt:      nil,
		Referer:            "",
		SessionReferer:     "",
		ContentReferer:     "",
		AbMask:             []string{},
		Timestamp:          time.Now(),
	}
	info := handle(update)
	// fmt.Println("Info from bot: ", info)
	event.FromBot = info.fromBot
	event.Data = info.data
	event.DataLowCardinality = info.dataLowCardinality
	event.DataInt = info.dataInt
	event.DataFlags = info.dataFlags
	event.ChatID = info.chatID
	event.UserID = info.userID
	event.Timestamp = info.timestamp

	if info.chatID != 0 {
		_, okUser := e.Users[info.chatID]
		if okUser {
			info.chatType = "private"
		}
		_, okChat := e.Chats[info.chatID]
		if okChat {
			info.chatType = "group"
		}

		channel, okChannel := e.Channels[info.chatID]
		if okChannel && channel.GetBroadcast() {
			info.chatType = "channel"
		}
		if okChannel && channel.GetMegagroup() {
			info.chatType = "supergroup"
		}
	}
	event.ChatType = info.chatType

	if !info.ignoreUpdate {
		if event.UserID != 0 {
			u.addUserInfoToEvent(ctx, &event, info, e)
		}
		u.clickCh <- &event
	}

	// fmt.Println("Update from bot: ", update.TypeName())
	// fmt.Println("Entities from bot: ", e)
	u.logger.Info(fmt.Sprint("Event from bot: ", event))

	// Update available chats and channels members
	if err := u.highLevelDispatch(ctx, e, update, info); err != nil {
		u.logger.Error("highLevelDispatch", zap.Error(err))
	}

	// time.Sleep(time.Second * 20)
	// fmt.Println(update)
	return nil
}

func (u *UpdateDispatcher) addUserInfoToEvent(ctx context.Context, event *database.Event, info *ExtractedInfo, e Entities) error {
	// u.keymutex.LockID(uint(event.UserID))
	// defer u.keymutex.UnlockID(uint(event.UserID))

	if user, okUser := e.Users[event.UserID]; okUser {
		event.Language, _ = user.GetLangCode()
	}

	userDb := database.User{BotID: event.BotID, UserID: event.UserID}
	if tx := u.db.Where(&userDb).First(&userDb); tx.Error != nil {
		if tx.Error != gorm.ErrRecordNotFound {
			return tx.Error
		}
		userDb.UserID = event.UserID
		userDb.BotID = event.BotID
		userDb.FirstActionTime = info.timestamp
		userDb.LastActionTime = info.timestamp
		userDb.RefererID = info.referer
		userDb.SessionID = int16(1)
		userDb.SessionRefererID = info.referer
		u.db.Create(&userDb)
	} else if info.updateSession {
		if userDb.LastActionTime.Before(info.timestamp.Add(-time.Minute * 5)) {
			userDb.SessionID++
			userDb.SessionRefererID = info.referer
		}
		userDb.LastActionTime = info.timestamp
		tx.Save(&userDb)
	}

	event.SessionID = userDb.SessionID
	event.Referer = userDb.RefererID
	event.SessionReferer = userDb.SessionRefererID
	event.UserCreatedAt = &userDb.FirstActionTime
	return nil
}

func (u *UpdateDispatcher) highLevelDispatch(ctx context.Context, e Entities, update tg.UpdateClass, info *ExtractedInfo) error {
	switch upd := update.(type) {
	case *tg.UpdateNewMessage:
		if err := u.handleMessage(ctx, upd.Message, info); err != nil {
			return err
		}
	case *tg.UpdateNewChannelMessage:
		if err := u.handleMessage(ctx, upd.Message, info); err != nil {
			return err
		}
	case *tg.UpdateChannelParticipant:
		oldMember, okOld := upd.GetPrevParticipant()
		newMember, okNew := upd.GetNewParticipant()
		okOld = okOld && oldMember.TypeID() != tg.ChannelParticipantLeftTypeID && oldMember.TypeID() != tg.ChannelParticipantBannedTypeID
		okNew = okNew && newMember.TypeID() != tg.ChannelParticipantLeftTypeID && newMember.TypeID() != tg.ChannelParticipantBannedTypeID

		if upd.UserID == u.botId {
			if err := u.updateChat(ctx, info, okNew, okOld && !okNew); err != nil {
				return err
			}
		}
		url := ""
		invite, okInvite := upd.GetInvite()
		if okInvite {
			switch inv := invite.(type) {
			case *tg.ChatInviteExported:
				url = inv.Link
			}
		}
		if err := u.updateChatMember(ctx, upd.ChannelID, upd.UserID, info, okNew && !okOld, okOld && !okNew, url, upd.ActorID); err != nil {
			return err
		}
	case *tg.UpdateChatParticipant:
		_, okOld := upd.GetPrevParticipant()
		_, okNew := upd.GetNewParticipant()
		if upd.UserID == u.botId {
			if err := u.updateChat(ctx, info, okNew, okOld && !okNew); err != nil {
				return err
			}
		}
		url := ""
		invite, okInvite := upd.GetInvite()
		if okInvite {
			switch inv := invite.(type) {
			case *tg.ChatInviteExported:
				url = inv.Link
			}
		}
		if err := u.updateChatMember(ctx, upd.ChatID, upd.UserID, info, okNew && !okOld, okOld && !okNew, url, upd.ActorID); err != nil {
			return err
		}
	case *tg.UpdateChatParticipantAdd:
	case *tg.UpdateChatParticipantDelete:
	case *tg.UpdateChatParticipants:
	case *tg.UpdateBotStopped:
		if err := u.updateChat(ctx, info, !upd.Stopped, upd.Stopped); err != nil {
			return err
		}
	}

	if info.chatID != 0 && info.userID != 0 && info.chatID != info.userID {
		if err := u.updateChatMember(ctx, info.chatID, info.userID, info, false, false, "", 0); err != nil {
			return err
		}
	}
	if info.chatID != 0 {
		if err := u.updateChat(ctx, info, false, false); err != nil {
			return err
		}
	}

	return nil
}

func (u *UpdateDispatcher) handleMessage(ctx context.Context, message tg.MessageClass, info *ExtractedInfo) error {
	switch m := message.(type) {
	case *tg.MessageEmpty:
		return nil
	case *tg.MessageService:
		switch action := m.Action.(type) {
		case *tg.MessageActionBotAllowed:
			return u.updateChat(ctx, info, true, false)
		case *tg.MessageActionChatAddUser:
			for _, userID := range action.Users {
				if err := u.updateChatMember(ctx, info.chatID, userID, info, true, false, "", info.userID); err != nil {
					return err
				}
			}
		case *tg.MessageActionChatJoinedByLink:
			return u.updateChatMember(ctx, info.chatID, info.userID, info, true, false, "", action.InviterID)
		case *tg.MessageActionChatJoinedByRequest:
			return u.updateChatMember(ctx, info.chatID, info.userID, info, true, false, "", 0)
		case *tg.MessageActionChatDeleteUser:
			return u.updateChatMember(ctx, info.chatID, action.UserID, info, false, true, "", info.userID)
		case *tg.MessageActionChatMigrateTo:
			// fmt.Println("Migrate to", action.ChannelID)
			return u.updateChatID(ctx, info.chatID, action.ChannelID)
		}
		return nil
	case *tg.Message:
		if m.PeerID.TypeID() == tg.PeerUserTypeID || m.Out {
			return u.updateChat(ctx, info, true, false)
		}
		return nil
	default:
		panic("Unknown message type")
	}
}

func max(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func (u *UpdateDispatcher) updateChat(ctx context.Context, info *ExtractedInfo, canWrite bool, ban bool) error {
	// u.keymutex.LockID(uint(info.chatID))
	// defer u.keymutex.UnlockID(uint(info.chatID))

	chat := database.Chat{BotID: u.botId, ChatID: info.chatID}
	tx := u.db.Where(&chat).First(&chat)
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return tx.Error
	}
	if tx.Error == gorm.ErrRecordNotFound {
		chat.ChatType = info.chatType
		chat.FirstActionTime = info.timestamp
		chat.LastActionTime = info.timestamp
		chat.LastUpdateTime = info.timestamp
		chat.RefererID = info.referer
		chat.CanWrite = canWrite
		chat.WasBanned = ban
		return u.db.Create(&chat).Error
	}

	chat.LastActionTime = max(chat.LastActionTime, info.timestamp)
	if !info.timestamp.Before(chat.LastUpdateTime) && info.chatType != "" && chat.ChatType != info.chatType {
		chat.ChatType = info.chatType
		chat.LastUpdateTime = info.timestamp
	}
	if !info.timestamp.Before(chat.LastUpdateTime) && (canWrite || ban) {
		chat.CanWrite = !ban && (chat.CanWrite || canWrite)
		chat.WasBanned = chat.WasBanned || ban
		chat.LastUpdateTime = info.timestamp
	}

	return tx.Save(&chat).Error
}

func (u *UpdateDispatcher) updateChatID(ctx context.Context, oldID int64, newID int64) error {
	if oldID == newID {
		return nil
	}

	err := u.db.Transaction(func(tx *gorm.DB) error {
		chat := database.Chat{BotID: u.botId, ChatID: oldID}
		err := tx.Where(&chat).First(&chat).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		newChat := database.Chat{BotID: u.botId, ChatID: newID}
		err = tx.Where(&newChat).First(&newChat).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			newChat.LastActionTime = chat.LastActionTime
			newChat.LastUpdateTime = chat.LastUpdateTime
		}
		newChat.MigratedFrom = chat.ChatID
		newChat.FirstActionTime = chat.FirstActionTime
		newChat.RefererID = chat.RefererID
		newChat.CanWrite = chat.CanWrite
		newChat.WasBanned = chat.WasBanned
		if err := tx.Save(&newChat).Error; err != nil {
			return err
		}
		// Delete old chat using primary key
		tx = tx.Delete(&chat)
		return tx.Error
	})
	return err
}

func (u *UpdateDispatcher) updateChatMember(
	ctx context.Context,
	chatID int64,
	memberID int64,
	info *ExtractedInfo,
	join bool,
	leave bool,
	joinUrl string,
	actorId int64,
) error {
	// This function can be called concurrently from different bots, so we need to handle dublicate key error
	err := u.updateChatMemberLocked(ctx, chatID, memberID, info, join, leave, joinUrl, actorId)
	if err == gorm.ErrDuplicatedKey {
		return u.updateChatMemberLocked(ctx, chatID, memberID, info, join, leave, joinUrl, actorId)
	}
	return err
}

func (u *UpdateDispatcher) updateChatMemberLocked(
	ctx context.Context,
	chatID int64,
	memberID int64,
	info *ExtractedInfo,
	join bool,
	leave bool,
	joinUrl string,
	actorId int64,
) error {
	// u.keymutex.LockID(uint(chatID))
	// defer u.keymutex.UnlockID(uint(chatID))

	chatMember := database.ChatMember{ChatID: chatID, UserID: memberID}
	tx := u.db.Where(&chatMember).First(&chatMember)
	if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
		return tx.Error
	}
	if tx.Error == gorm.ErrRecordNotFound && join {
		chatMember.FirstJoinTime = &info.timestamp
		chatMember.LastJoinTime = &info.timestamp
		chatMember.LastActionTime = info.timestamp
		chatMember.JoinUrl = joinUrl
		chatMember.FirstJoinActorId = actorId
		chatMember.LastJoinActorId = actorId
		chatMember.IsMember = !leave
		return u.db.Create(&chatMember).Error
	}
	if tx.Error == gorm.ErrRecordNotFound && leave {
		chatMember.LastLeaveTime = &info.timestamp
		chatMember.LastActionTime = info.timestamp
		chatMember.LastLeaveActorId = actorId
		chatMember.IsMember = false
		return u.db.Create(&chatMember).Error
	}
	if tx.Error == gorm.ErrRecordNotFound {
		chatMember.LastActionTime = info.timestamp
		chatMember.IsMember = true
		return u.db.Create(&chatMember).Error
	}

	canApplyJoin := chatMember.LastJoinTime == nil || !info.timestamp.Before(*chatMember.LastJoinTime)
	canApplyLeave := chatMember.LastLeaveTime == nil || !info.timestamp.Before(*chatMember.LastLeaveTime)
	canApply := canApplyJoin && canApplyLeave
	if canApply && join {
		chatMember.LastJoinTime = &info.timestamp
		chatMember.LastJoinActorId = actorId
		chatMember.IsMember = true
		if chatMember.JoinUrl == "" {
			chatMember.JoinUrl = joinUrl
		}
	}
	if canApply && leave {
		chatMember.LastLeaveTime = &info.timestamp
		chatMember.LastLeaveActorId = actorId
		chatMember.IsMember = false
	}

	chatMember.LastActionTime = max(info.timestamp, chatMember.LastActionTime)
	return tx.Save(&chatMember).Error
}
