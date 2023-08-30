package bot

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gotd/td/tg"
)

type ExtractedInfo struct {
	ignoreUpdate       bool
	fromBot            bool
	userID             int64
	chatID             int64
	chatType           string
	updateSession      bool
	data               []string
	dataLowCardinality []string
	dataInt            []int64
	dataFlags          []bool
	referer            string
	timestamp          time.Time
}

func getPeerID(peer tg.PeerClass) int64 {
	switch peerID := peer.(type) {
	case *tg.PeerUser:
		return peerID.UserID
	case *tg.PeerChat:
		return peerID.ChatID
	case *tg.PeerChannel:
		return peerID.ChannelID
		// This is not needed because it will be a nightmare to handle
		// mult := int64(1)
		// for mult <= peerID.ChannelID {
		// 	mult *= 10
		// }
		// return -100*mult - peerID.ChannelID
	default:
		panic("Invalid peer")
	}
}

func dataFromMessage(message tg.MessageClass, info *ExtractedInfo) *ExtractedInfo {
	info.data = make([]string, 0)
	info.dataLowCardinality = make([]string, 4)
	info.dataInt = make([]int64, 2)
	info.dataFlags = make([]bool, 2)

	switch m := message.(type) {
	case *tg.MessageEmpty:
		info.dataInt[0] = int64(m.GetID())
		peerID, ok := m.GetPeerID()
		info.dataLowCardinality[0] = "MessageEmpty"
		info.dataLowCardinality[1] = "Empty"
		if ok {
			info.chatID = getPeerID(peerID)
			info.dataLowCardinality[2] = peerID.TypeName()
			info.dataLowCardinality[3] = peerID.TypeName()
		}
		if !ok || peerID.TypeID() == tg.PeerChannelTypeID {
			info.ignoreUpdate = true
		}
		return info
	case *tg.MessageService:
		info.fromBot = m.GetOut()
		info.updateSession = !info.fromBot

		info.dataInt[0] = int64(m.GetID())
		// info.dataInt = append(info.dataInt, viaBot)

		info.dataFlags[0] = false
		info.dataFlags[1] = m.GetMentioned()

		info.timestamp = time.Unix(int64(m.GetDate()), 0)

		info.dataLowCardinality[0] = "MessageService"
		info.dataLowCardinality[1] = m.Action.TypeName()

		peer := m.GetPeerID()
		from, okFrom := m.GetFromID()

		if okFrom {
			info.chatID = getPeerID(peer)
			info.userID = getPeerID(from)
			info.dataLowCardinality[2] = peer.TypeName()
			info.dataLowCardinality[3] = from.TypeName()
		} else {
			info.chatID = getPeerID(peer)
			info.userID = getPeerID(peer)
			info.dataLowCardinality[2] = peer.TypeName()
			info.dataLowCardinality[3] = peer.TypeName()
		}

		return info
	case *tg.Message:
		info.fromBot = m.GetOut()
		info.updateSession = !info.fromBot

		info.dataInt[0] = int64(m.GetID())
		viaBot, okViaBot := m.GetViaBotID()
		if okViaBot {
			info.dataInt[1] = viaBot
		}

		info.dataFlags[0] = m.GetEditHide()
		info.dataFlags[1] = m.GetMentioned()

		editDate, okEditDate := m.GetEditDate()
		if okEditDate {
			info.timestamp = time.Unix(int64(editDate), 0)
		} else {
			info.timestamp = time.Unix(int64(m.GetDate()), 0)
		}

		info.dataLowCardinality[0] = "Message"
		media, okMedia := m.GetMedia()
		if okMedia {
			info.dataLowCardinality[1] = media.TypeName()
		} else {
			info.dataLowCardinality[1] = "Text"
		}

		peer := m.GetPeerID()
		from, okFrom := m.GetFromID()

		if okFrom {
			info.chatID = getPeerID(peer)
			info.userID = getPeerID(from)
			info.dataLowCardinality[2] = peer.TypeName()
			info.dataLowCardinality[3] = from.TypeName()
		} else {
			info.chatID = getPeerID(peer)
			info.userID = getPeerID(peer)
			info.dataLowCardinality[2] = peer.TypeName()
			info.dataLowCardinality[3] = peer.TypeName()
		}

		if !m.Post && !okViaBot && !m.Mentioned && m.PeerID.TypeID() == tg.PeerChannelTypeID {
			info.ignoreUpdate = true
		}
		return info
	default:
		panic("Invalid message")
	}
}

func handle(update tg.UpdateClass) *ExtractedInfo {
	info := ExtractedInfo{
		ignoreUpdate:       false,
		fromBot:            false,
		userID:             0,
		chatID:             0,
		chatType:           "",
		updateSession:      false,
		data:               []string{},
		dataLowCardinality: []string{},
		dataInt:            []int64{},
		dataFlags:          []bool{},
		referer:            "",
		timestamp:          time.Now(),
	}

	switch u := update.(type) {
	case *tg.UpdateNewMessage:
		return dataFromMessage(u.Message, &info)
	case *tg.UpdateMessageID:
	case *tg.UpdateDeleteMessages:
	case *tg.UpdateUserTyping:

	case *tg.UpdateChatUserTyping:

	case *tg.UpdateChatParticipants:

	case *tg.UpdateUserStatus:

	case *tg.UpdateUserName:

	case *tg.UpdateNewEncryptedMessage: // Not needed
	case *tg.UpdateEncryptedChatTyping: // Non needed
	case *tg.UpdateEncryption: // Non needed
	case *tg.UpdateEncryptedMessagesRead: // Non needed
	case *tg.UpdateChatParticipantAdd:
	case *tg.UpdateChatParticipantDelete:
	case *tg.UpdateDCOptions:
	case *tg.UpdateNotifySettings: // Non needed
	case *tg.UpdateServiceNotification: // Non needed
	case *tg.UpdatePrivacy: // Non needed
	case *tg.UpdateUserPhone:
	case *tg.UpdateReadHistoryInbox: // Non needed
	case *tg.UpdateReadHistoryOutbox: // Non needed
	case *tg.UpdateWebPage:
	case *tg.UpdateReadMessagesContents:
	case *tg.UpdateChannelTooLong:
	case *tg.UpdateChannel:
	case *tg.UpdateNewChannelMessage:
		return dataFromMessage(u.Message, &info)
	case *tg.UpdateReadChannelInbox: // Non needed
	case *tg.UpdateDeleteChannelMessages:
	case *tg.UpdateChannelMessageViews:
		info.chatID = u.ChannelID
		info.dataInt = append(info.dataInt, int64(u.ID))
		info.dataInt = append(info.dataInt, int64(u.Views))
		return &info
	case *tg.UpdateChatParticipantAdmin:
	case *tg.UpdateNewStickerSet:
	case *tg.UpdateStickerSetsOrder:
	case *tg.UpdateStickerSets:
	case *tg.UpdateSavedGifs:
	case *tg.UpdateBotInlineQuery:
		info.userID = u.UserID
		info.fromBot = false
		info.updateSession = true

		peerType, okPeerType := u.GetPeerType()
		inlineChatType := ""
		if okPeerType {
			switch peerType.TypeID() {
			case tg.InlineQueryPeerTypeBroadcastTypeID:
				info.chatType = "channel"
			case tg.InlineQueryPeerTypeChatTypeID:
				info.chatType = "group"
			case tg.InlineQueryPeerTypeMegagroupTypeID:
				info.chatType = "supergroup"
			case tg.InlineQueryPeerTypePMTypeID:
				info.chatType = "private"
			case tg.InlineQueryPeerTypeBotPMTypeID:
				info.chatType = "private"
			case tg.InlineQueryPeerTypeSameBotPMTypeID:
				info.chatType = "private"
			}
			inlineChatType = peerType.TypeName()
		}
		info.dataLowCardinality = append(info.dataLowCardinality, inlineChatType) // Previously was chatType
		info.dataLowCardinality = append(info.dataLowCardinality, u.Offset)
		info.dataInt = append(info.dataInt, int64(utf8.RuneCountInString(u.Query)))
		_, okGeo := u.GetGeo()
		info.dataFlags = append(info.dataFlags, okGeo)
		return &info
	case *tg.UpdateBotInlineSend:
		info.userID = u.UserID
		info.fromBot = true
		info.updateSession = true
		info.dataLowCardinality = append(info.dataLowCardinality, u.ID)
		info.dataInt = append(info.dataInt, int64(utf8.RuneCountInString(u.Query)))
		_, okGeo := u.GetGeo()
		_, okMsgID := u.GetMsgID()
		info.dataFlags = append(info.dataFlags, okGeo)
		info.dataFlags = append(info.dataFlags, okMsgID)
		return &info
	case *tg.UpdateEditChannelMessage:
		return dataFromMessage(u.Message, &info)
	case *tg.UpdateBotCallbackQuery:
		info.userID = u.UserID
		info.chatID = getPeerID(u.Peer)
		info.fromBot = false
		info.updateSession = true
		info.dataInt = append(info.dataInt, u.ChatInstance)
		info.dataInt = append(info.dataInt, int64(u.MsgID))
		info.dataLowCardinality = append(info.dataLowCardinality, u.Peer.TypeName())
		gameName, okGameName := u.GetGameShortName()
		if okGameName {
			info.dataLowCardinality = append(info.dataLowCardinality, gameName)
		}
		return &info
	case *tg.UpdateEditMessage:
		return dataFromMessage(u.Message, &info)
	case *tg.UpdateInlineBotCallbackQuery:
		info.userID = u.UserID
		info.fromBot = false
		info.updateSession = true
		info.dataInt = append(info.dataInt, u.ChatInstance)
		gameName, okGameName := u.GetGameShortName()
		if okGameName {
			info.dataLowCardinality = append(info.dataLowCardinality, gameName)
		}
		return &info
	case *tg.UpdateReadChannelOutbox: // not needed
	case *tg.UpdateDraftMessage: // not needed
	case *tg.UpdateReadFeaturedStickers:
	case *tg.UpdateRecentStickers: // not needed
	case *tg.UpdateConfig:
	case *tg.UpdatePtsChanged:
	case *tg.UpdateChannelWebPage:
	case *tg.UpdateDialogPinned: // not needed
	case *tg.UpdatePinnedDialogs: // not needed
	case *tg.UpdateBotWebhookJSON:
	case *tg.UpdateBotWebhookJSONQuery:
	case *tg.UpdateBotShippingQuery:
	case *tg.UpdateBotPrecheckoutQuery:
	case *tg.UpdatePhoneCall: // not needed
	case *tg.UpdateLangPackTooLong:
	case *tg.UpdateLangPack:
	case *tg.UpdateFavedStickers: // not needed
	case *tg.UpdateChannelReadMessagesContents:
	case *tg.UpdateContactsReset: // not needed
	case *tg.UpdateChannelAvailableMessages:
	case *tg.UpdateDialogUnreadMark: // not needed
	case *tg.UpdateMessagePoll:
	case *tg.UpdateChatDefaultBannedRights:
	case *tg.UpdateFolderPeers: // not needed
	case *tg.UpdatePeerSettings:
	case *tg.UpdatePeerLocated:
	case *tg.UpdateNewScheduledMessage: // not needed
	case *tg.UpdateDeleteScheduledMessages: // not needed
	case *tg.UpdateTheme: // not needed
	case *tg.UpdateGeoLiveViewed:
	case *tg.UpdateLoginToken:
	case *tg.UpdateMessagePollVote:
	case *tg.UpdateDialogFilter:
	case *tg.UpdateDialogFilterOrder:
	case *tg.UpdateDialogFilters:
	case *tg.UpdatePhoneCallSignalingData:
	case *tg.UpdateChannelMessageForwards:
	case *tg.UpdateReadChannelDiscussionInbox:
	case *tg.UpdateReadChannelDiscussionOutbox:
	case *tg.UpdatePeerBlocked:
	case *tg.UpdateChannelUserTyping:
	case *tg.UpdatePinnedMessages:
	case *tg.UpdatePinnedChannelMessages:
	case *tg.UpdateChat:
	case *tg.UpdateGroupCallParticipants:
	case *tg.UpdateGroupCall:
	case *tg.UpdatePeerHistoryTTL:
	case *tg.UpdateChatParticipant:
		info.fromBot = (u.UserID == u.ActorID)
		info.updateSession = false
		info.chatID = u.ChatID
		info.userID = u.UserID
		_, okOld := u.GetPrevParticipant()
		info.dataFlags = append(info.dataFlags, okOld)
		_, okNew := u.GetNewParticipant()
		info.dataFlags = append(info.dataFlags, okNew)
		invite, okInvite := u.GetInvite()
		if okInvite && invite.TypeID() == tg.ChatInviteExportedTypeID {
			// fmt.Println(invite.(*tg.ChatInviteExported))
			invite_hash := strings.Replace(invite.(*tg.ChatInviteExported).Link, "https://t.me/", "", 1)
			info.dataLowCardinality = append(info.dataLowCardinality, invite_hash)
		}
		info.timestamp = time.Unix(int64(u.Date), 0)
		return &info
	case *tg.UpdateChannelParticipant:
		info.fromBot = (u.UserID == u.ActorID)
		info.updateSession = false
		info.chatID = u.ChannelID
		info.userID = u.UserID
		_, okOld := u.GetPrevParticipant()
		info.dataFlags = append(info.dataFlags, okOld)
		_, okNew := u.GetNewParticipant()
		info.dataFlags = append(info.dataFlags, okNew)
		info.dataFlags = append(info.dataFlags, u.ViaChatlist)
		invite, okInvite := u.GetInvite()
		if okInvite && invite.TypeID() == tg.ChatInviteExportedTypeID {
			// fmt.Println(invite.(*tg.ChatInviteExported))
			invite_hash := strings.Replace(invite.(*tg.ChatInviteExported).Link, "https://t.me/", "", 1)
			info.dataLowCardinality = append(info.dataLowCardinality, invite_hash)
		}
		info.timestamp = time.Unix(int64(u.Date), 0)
		return &info
	case *tg.UpdateBotStopped:
		info.fromBot = false
		info.updateSession = false
		info.chatID = u.UserID
		info.userID = u.UserID
		info.dataFlags = append(info.dataFlags, u.Stopped)
		info.timestamp = time.Unix(int64(u.Date), 0)
		return &info
	case *tg.UpdateGroupCallConnection:
	case *tg.UpdateBotCommands:
	case *tg.UpdatePendingJoinRequests:
	case *tg.UpdateBotChatInviteRequester:
	case *tg.UpdateMessageReactions:
	case *tg.UpdateAttachMenuBots:
	case *tg.UpdateWebViewResultSent:
	case *tg.UpdateBotMenuButton:
	case *tg.UpdateSavedRingtones:
	case *tg.UpdateTranscribedAudio:
	case *tg.UpdateReadFeaturedEmojiStickers:
	case *tg.UpdateUserEmojiStatus:
	case *tg.UpdateRecentEmojiStatuses:
	case *tg.UpdateRecentReactions:
	case *tg.UpdateMoveStickerSetToTop:
	case *tg.UpdateMessageExtendedMedia:
	case *tg.UpdateChannelPinnedTopic:

	case *tg.UpdateChannelPinnedTopics:

	case *tg.UpdateUser:
		info.fromBot = false
		info.updateSession = false
		info.userID = u.UserID
		return &info
	case *tg.UpdateAutoSaveSettings: // Non needed
	case *tg.UpdateGroupInvitePrivacyForbidden:
	default:
	}

	return &info
}
