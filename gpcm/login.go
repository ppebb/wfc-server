package gpcm

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"github.com/jackc/pgx/v4/pgxpool"
	"log"
	"strconv"
	"strings"
	"wwfc/common"
	"wwfc/database"
)

func generateResponse(gpcmChallenge, nasChallenge, authToken, clientChallenge string) string {
	hasher := md5.New()
	hasher.Write([]byte(nasChallenge))
	str := hex.EncodeToString(hasher.Sum(nil))
	str += "                                                "
	str += authToken
	str += clientChallenge
	str += gpcmChallenge
	str += hex.EncodeToString(hasher.Sum(nil))

	_hasher := md5.New()
	_hasher.Write([]byte(str))
	return hex.EncodeToString(_hasher.Sum(nil))
}

func generateProof(gpcmChallenge, nasChallenge, authToken, clientChallenge string) string {
	return generateResponse(clientChallenge, nasChallenge, authToken, gpcmChallenge)
}

func Login(session *GameSpySession, pool *pgxpool.Pool, ctx context.Context, command common.GameSpyCommand, challenge string) (string, bool) {
	if session.LoggedIn {
		log.Fatalf("Attempt to login twice")
	}

	// TODO: Validate login token with one in database
	authToken := command.OtherValues["authtoken"]
	response := generateResponse(challenge, "0qUekMb4", authToken, command.OtherValues["challenge"])
	if response != command.OtherValues["response"] {
		// TODO: Return an error
		log.Fatalf("response mismatch")
	}

	proof := generateProof(challenge, "0qUekMb4", command.OtherValues["authtoken"], command.OtherValues["challenge"])

	// Perform the login with the database.
	// TODO: Check valid result
	user, ok := database.LoginUserToGPCM(pool, ctx, authToken)
	if !ok {
		// TODO: Return an error
		log.Fatalf("GPCM login error")
	}
	session.User = user

	loginTicket := strings.Replace(base64.StdEncoding.EncodeToString([]byte(common.RandomString(16))), "=", "_", -1)
	// Now initiate the session
	_ = database.CreateSession(pool, ctx, session.User.ProfileId, loginTicket)

	session.LoggedIn = true
	session.ModuleName += ":" + strconv.FormatInt(int64(session.User.ProfileId), 10)
	session.ModuleName += "/" + common.CalcFriendCodeString(session.User.ProfileId, "RMCJ")

	return common.CreateGameSpyMessage(common.GameSpyCommand{
		Command:      "lc",
		CommandValue: "2",
		OtherValues: map[string]string{
			"sesskey":    "199714190",
			"proof":      proof,
			"userid":     strconv.FormatInt(session.User.UserId, 10),
			"profileid":  strconv.FormatInt(int64(session.User.ProfileId), 10),
			"uniquenick": session.User.UniqueNick,
			"lt":         loginTicket,
			"id":         command.OtherValues["id"],
		},
	}), true
}
