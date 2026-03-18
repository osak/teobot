package mastodon

type Client interface {
	VerifyCredentials() (*Account, error)
	GetStatus(id string) (*Status, error)
	GetReplyTree(id string) (*Context, error)
	PostStatus(content string, opt *PostStatusOpt) (*Status, error)
	GetAllNotifications(opt *GetAllNotificationsOpt) ([]*Notification, error)
	UploadImage(imageData []byte) (*MediaAttachment, error)
	GetImage(id string) (*MediaAttachment, error)
}
