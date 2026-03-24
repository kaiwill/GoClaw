package channels

const (
	MessageTypeNone  = 0
	MessageTypeUser  = 1
	MessageTypeBot   = 2
)

const (
	MessageItemTypeNone  = 0
	MessageItemTypeText  = 1
	MessageItemTypeImage = 2
	MessageItemTypeVoice = 3
	MessageItemTypeFile  = 4
	MessageItemTypeVideo = 5
)

const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

const (
	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4
)

type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

type TextItem struct {
	Text string `json:"text,omitempty"`
}

type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AesKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

type ImageItem struct {
	Media      *CDNMedia `json:"media,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
	AesKey     string    `json:"aeskey,omitempty"`
	URL        string    `json:"url,omitempty"`
	MidSize    int       `json:"mid_size,omitempty"`
	ThumbSize  int       `json:"thumb_size,omitempty"`
}

type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int       `json:"playtime,omitempty"`
	Text          string    `json:"text,omitempty"`
}

type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

type VideoItem struct {
	Media      *CDNMedia `json:"media,omitempty"`
	VideoSize  int       `json:"video_size,omitempty"`
	PlayLength int       `json:"play_length,omitempty"`
	VideoMD5   string    `json:"video_md5,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize  int       `json:"thumb_size,omitempty"`
}

type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

type MessageItem struct {
	Type          int          `json:"type,omitempty"`
	CreateTimeMs  int64        `json:"create_time_ms,omitempty"`
	UpdateTimeMs  int64        `json:"update_time_ms,omitempty"`
	IsCompleted   bool         `json:"is_completed,omitempty"`
	MsgID         string       `json:"msg_id,omitempty"`
	RefMsg        *RefMessage  `json:"ref_msg,omitempty"`
	TextItem      *TextItem    `json:"text_item,omitempty"`
	ImageItem     *ImageItem   `json:"image_item,omitempty"`
	VoiceItem     *VoiceItem   `json:"voice_item,omitempty"`
	FileItem      *FileItem    `json:"file_item,omitempty"`
	VideoItem     *VideoItem   `json:"video_item,omitempty"`
}

type WeixinMessage struct {
	Seq          int            `json:"seq,omitempty"`
	MessageID    int64          `json:"message_id,omitempty"`
	FromUserID   string         `json:"from_user_id,omitempty"`
	ToUserID     string         `json:"to_user_id,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	CreateTimeMs int64          `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64          `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64          `json:"delete_time_ms,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	GroupID      string         `json:"group_id,omitempty"`
	MessageType  int            `json:"message_type,omitempty"`
	MessageState int            `json:"message_state,omitempty"`
	ItemList     []*MessageItem `json:"item_list,omitempty"`
	ContextToken string         `json:"context_token,omitempty"`
}

type GetUpdatesReq struct {
	GetUpdatesBuf string `json:"get_updates_buf,omitempty"`
}

type GetUpdatesResp struct {
	Ret                 int              `json:"ret,omitempty"`
	ErrCode             int              `json:"errcode,omitempty"`
	ErrMsg              string           `json:"errmsg,omitempty"`
	Msgs                []*WeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf       string           `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMs int             `json:"longpolling_timeout_ms,omitempty"`
}

type SendMessageReq struct {
	Msg *WeixinMessage `json:"msg,omitempty"`
}

type SendTypingReq struct {
	ILinkUserID   string `json:"ilink_user_id,omitempty"`
	TypingTicket  string `json:"typing_ticket,omitempty"`
	Status        int    `json:"status,omitempty"`
}

type GetUploadUrlReq struct {
	FileKey         string `json:"filekey,omitempty"`
	MediaType       int    `json:"media_type,omitempty"`
	ToUserID        string `json:"to_user_id,omitempty"`
	RawSize         int    `json:"rawsize,omitempty"`
	RawFileMD5      string `json:"rawfilemd5,omitempty"`
	FileSize        int    `json:"filesize,omitempty"`
	ThumbRawSize    int    `json:"thumb_rawsize,omitempty"`
	ThumbRawFileMD5 string `json:"thumb_rawfilemd5,omitempty"`
	ThumbFileSize   int    `json:"thumb_filesize,omitempty"`
	NoNeedThumb     bool   `json:"no_need_thumb,omitempty"`
	AesKey          string `json:"aeskey,omitempty"`
}

type GetUploadUrlResp struct {
	UploadParam     string `json:"upload_param,omitempty"`
	ThumbUploadParam string `json:"thumb_upload_param,omitempty"`
}

type GetConfigResp struct {
	Ret          int    `json:"ret,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket,omitempty"`
}

type QRCodeResponse struct {
	QRCode         string `json:"qrcode,omitempty"`
	QRCodeImgContent string `json:"qrcode_img_content,omitempty"`
}

type QRStatusResponse struct {
	Status      string `json:"status,omitempty"`
	BotToken    string `json:"bot_token,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	BaseURL     string `json:"baseurl,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
}

type WeixinAccountData struct {
	Token    string `json:"token,omitempty"`
	SavedAt  string `json:"savedAt,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
	UserID   string `json:"userId,omitempty"`
}
