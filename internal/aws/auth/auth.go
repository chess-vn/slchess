package auth

func MustAuth(authorizer map[string]interface{}) string {
	jwt, ok := authorizer["jwt"].(map[string]interface{})
	if !ok {
		panic("no jwt")
	}
	v, exists := jwt["claims"]
	if !exists {
		panic("no authorizer claims")
	}
	claims, ok := v.(map[string]interface{})
	if !ok {
		panic("claims must be of type map")
	}
	userId, ok := claims["sub"].(string)
	if !ok {
		panic("invalid sub")
	}
	return userId
}
