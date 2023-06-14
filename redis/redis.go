package redis

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/Radictionary/website/models"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

var rdb *redis.Client

func InitRedisConnection() error {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	redisAddr := os.Getenv("REDIS_SERVER_ADDR")
	redisPassword := os.Getenv("REDIS_SERVER_PASSWORD")
	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	_, err = rdb.Ping(context.Background()).Result()
	return err
}
func StoreData(key string, value string) error {
	ctx := context.Background()
	err := rdb.Set(ctx, key, value, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to store data: %s", err.Error())
	}
	return nil
}
func RetrieveData(key string) (string, error) {
	ctx := context.Background()
	value, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key '%s' does not exist", key)
	} else if err != nil {
		return "", fmt.Errorf("failed to retrieve data: %s", err.Error())
	}
	return value, nil
}
func HashStruct(packet models.PacketStruct) error {
	redisFields := map[string]interface{}{
		"Interface":    packet.Interface,
		"Protocol":     packet.Protocol,
		"SrcAddr":      packet.SrcAddr,
		"DstnAddr":     packet.DstnAddr,
		"Length":       packet.Length,
		"PacketNumber": packet.PacketNumber,
		"PacketDump":   packet.PacketDump,
		"PacketData": packet.PacketData,
		"Time":         packet.Time,
		"Err":          packet.Err,
		"Saved":        packet.Saved,
	}

	key := fmt.Sprintf("packet:%d", packet.PacketNumber)
	err := rdb.HMSet(context.Background(), key, redisFields).Err()
	if err != nil {
		return err
	}

	return nil
}

func RetrieveStruct(key string) (models.PacketStruct, error) {
	result, err := rdb.HGetAll(context.Background(), key).Result()
	if err != nil {
		return models.PacketStruct{}, err
	}

	packet := models.PacketStruct{
		Interface:    result["Interface"],
		Protocol:     result["Protocol"],
		SrcAddr:      result["SrcAddr"],
		DstnAddr:     result["DstnAddr"],
		Length:       convertStringToInt(result["Length"]),
		PacketNumber: convertStringToInt(result["PacketNumber"]),
		PacketDump:   result["PacketDump"],
		PacketData: []byte((result["PacketData"])),
		Time:         result["Time"],
		Err:          result["Err"],
		Saved:        convertStringToBool(result["Saved"]),
	}

	return packet, nil
}

func RecoverPackets() ([]models.PacketStruct, error) {
	var packets []models.PacketStruct
	iter := rdb.Scan(context.Background(), 0, "packet:*", 0).Iterator()
	for iter.Next(context.Background()) {
		key := iter.Val()
		packet, err := RetrieveStruct(key)
		if err != nil {
			return nil, err
		}
		if !packet.Saved {
			packets = append(packets, packet)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	// Sort packets by packet number
	sort.Slice(packets, func(i, j int) bool {
		return packets[i].PacketNumber < packets[j].PacketNumber
	})

	return packets, nil
}

func MarkAsSaved() error{
	packetsToSave, err := RecoverPackets()
	if err != nil {
		return err
	}
	for _, packet := range packetsToSave {
		packet.Saved = true
		HashStruct(packet)
	}
	return nil
}

func ClearPackets() error {
	cursor := uint64(0)
	var batchSize int64 = 1000
	for {
		keys, nextCursor, err := rdb.Scan(context.Background(), cursor, "packet:*", batchSize).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			err := rdb.Del(context.Background(), keys...).Err()
			if err != nil {
				return err
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	fmt.Println("CLEARED ALL PACKETS")
	return nil
}

func convertStringToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		fmt.Println("Error converting string to int:", err)
		return 0
	}
	return i
}

func convertStringToBool(s string) bool {
	i, err := strconv.ParseBool(s)
	if err != nil {
		fmt.Println("Error converting string to bool:", err)
	}
	return i
}
