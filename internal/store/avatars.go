package store

import "context"

const keyAvatarPfx = "avatar:"

// SetAvatar stores the uploaded photo's raw bytes (base64-encoded under
// the hood, same as thumbnails) and flips HasAvatar on the user record so
// the frontend knows to request it instead of falling back to the
// default icon.
func (r *Redis) SetAvatar(ctx context.Context, userID string, data []byte, contentType string) error {
	if err := r.SetBytes(ctx, keyAvatarPfx+userID, data, noExpiry); err != nil {
		return err
	}
	if err := r.SetString(ctx, keyAvatarPfx+userID+":type", contentType, noExpiry); err != nil {
		return err
	}

	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	user.HasAvatar = true
	return r.saveUser(ctx, user)
}

func (r *Redis) GetAvatar(ctx context.Context, userID string) ([]byte, string, bool) {
	data, ok := r.GetBytes(ctx, keyAvatarPfx+userID)
	if !ok {
		return nil, "", false
	}
	contentType, _ := r.GetString(ctx, keyAvatarPfx+userID+":type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, true
}
