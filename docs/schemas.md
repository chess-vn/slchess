---
id: Database-Design
aliases: []
tags: []
---

### User

```json
{
  "UserId": "player_123",
  "Username": "zuka",
  "Avatar": "s3://asdfdf",
  "Country": "Vietnam",
  "Elo": 1900

  "SubscriptionStatus": "active", # active/inactive
  "SubscriptionStartDate: "2025-01-01T00:00:00Z",
  
}
```

### Active Game

```json
{
  "GameId": "game_123",
  "Player1": "player_1_uid",
  "Player2": "player_2_uid",
  "Server": "192.168.0.2"
}
```

### Game result

- Update after both player disconnected or game ended (same thing)

```json
{
  "GameSessionId": "game_123",
  "Players": ["player_1_uid", "player_2_uid"],
  "StartTime": "2025-01-11T10:00:00Z",
  "EndTime": "2025-01-11T10:15:00Z",
  "Result": "WHITE_CHECKMATE",
  "Pgn": "s3://game-records/game_123"
  "Messages": "s3://messages/game_123"
}
```

After game ended, push the game replay data to S3

### Game state: -> Use AppSync to sync game state to Spectator

```json
{
  "gameId": "game_123",
  "players": {
    "player1": {"timeRemaining": 100000, "lastMoveTimestamp": None},
    "player2": {"timeRemaining": 100000, "lastMoveTimestamp": None},
  },
  "activePlayer": "player_uid",
  "increment": 2000,  # 2 seconds increment
  "delay": 0          # No delay
}
```

### Messages

```json
{
  "MessageId" mss_123
  "GameId": "game_123",
  "Sender": "player_1_uid",
  "Content": "Hello World",
  "CreatedAt": "2025-01-11T10:00:00Z"
}
```
