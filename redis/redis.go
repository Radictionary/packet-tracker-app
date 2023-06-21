package redis

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"

	"github.com/Radictionary/packy/models"
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
	// redisAddr := "localhost:6379"
	// redisPassword := ""
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
func HashStruct(packet models.PacketStruct, identifier string) error {
	redisFields := map[string]interface{}{
		"interface":    packet.Interface,
		"protocol":     packet.Protocol,
		"srcAddr":      packet.SrcAddr,
		"dstnAddr":     packet.DstnAddr,
		"length":       packet.Length,
		"packetNumber": packet.PacketNumber,
		"packetDump":   packet.PacketDump,
		"packetData":   packet.PacketData,
		"time":         packet.Time,
		"err":          packet.Err,
		"saved":        packet.Saved,
	}

	key := fmt.Sprintf("%v:%d", identifier, packet.PacketNumber)
	err := rdb.HMSet(context.Background(), key, redisFields).Err()
	if err != nil {
		return err
	}

	return nil
}

func RetrieveMap(key string) (map[string]string, error) {
	result, err := rdb.HGetAll(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func RetrieveStruct(key string) (models.PacketStruct, error) {
	result, err := rdb.HGetAll(context.Background(), key).Result()
	if err != nil {
		return models.PacketStruct{}, err
	}

	packet := models.PacketStruct{
		Interface:    result["interface"],
		Protocol:     result["protocol"],
		SrcAddr:      result["srcAddr"],
		DstnAddr:     result["dstnAddr"],
		Length:       convertStringToInt(result["length"]),
		PacketNumber: convertStringToInt(result["packetNumber"]),
		PacketDump:   result["packetDump"],
		PacketData:   []byte((result["packetData"])),
		Time:         result["time"],
		Err:          result["err"],
		Saved:        convertStringToBool(result["saved"]),
	}

	return packet, nil
}

func MarkAsSaved() error {
	packetsToSave, err := RecoverPackets("packet", nil)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for i := range packetsToSave {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			packet := &packetsToSave[index]
			packet.Saved = true
			HashStruct(*packet, "packet")
		}(i)
	}

	wg.Wait()
	return nil
}

func RecoverPackets(identifier string, messageChan chan models.PacketStruct) ([]models.PacketStruct, error) {
	iter := rdb.Scan(context.Background(), 0, identifier+":*", 0).Iterator()
	go func() {
		var protocol string
		packetsToRecover := CountHashesByPattern(identifier, rdb)
		if identifier == "packetsFromFile" {
			protocol  = "packetsFromFile"
		} else if identifier == "packet" {
			protocol = "unsavedPacket"
		}
		messageChan <- models.PacketStruct{
			Interface: "recover_packet_number",
			Length:    packetsToRecover,
			Protocol: protocol,
		}
	}()
	var packets []models.PacketStruct
	var wg sync.WaitGroup
	var mutex sync.Mutex

	for iter.Next(context.Background()) {
		packetNumberKey := iter.Val()

		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			packetData, err := rdb.HGetAll(context.Background(), key).Result()
			if err != nil {
				return
			}
			if convertStringToBool(packetData["saved"]) && packetData["interface"] != "N/A" {
				return
			}
			packet := models.PacketStruct{
				Interface:    packetData["interface"],
				Protocol:     packetData["protocol"],
				SrcAddr:      packetData["srcAddr"],
				DstnAddr:     packetData["dstnAddr"],
				Length:       convertStringToInt(packetData["length"]),
				PacketNumber: convertStringToInt(packetData["packetNumber"]),
				PacketDump:   packetData["packetDump"],
				PacketData:   []byte(packetData["packetData"]),
				Time:         packetData["time"],
				Err:          packetData["err"],
				Saved:        convertStringToBool(packetData["saved"]),
			}
			mutex.Lock()
			packets = append(packets, packet)
			mutex.Unlock()
		}(packetNumberKey)
	}
	wg.Wait()

	if err := iter.Err(); err != nil {
		return nil, err
	}

	// Sort packets by packet number
	sort.SliceStable(packets, func(i, j int) bool {
		return packets[i].PacketNumber < packets[j].PacketNumber
	})
	return packets, nil
}

func ClearPackets(identifier string) error {
	cursor := uint64(0)
	var batchSize int64 = 1000
	for {
		keys, nextCursor, err := rdb.Scan(context.Background(), cursor, identifier+":*", batchSize).Result()
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

func CountHashesByPattern(pattern string, client *redis.Client) int {
	keys, err := client.Keys(context.Background(), pattern + ":*").Result()
	if err != nil {
		panic(err)
	}

	// Create a channel to receive key types
	keyTypesCh := make(chan string, len(keys))

	// Wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	wg.Add(len(keys))

	// Spawn goroutines to fetch key types
	for _, key := range keys {
		go func(k string) {
			defer wg.Done()
			keyType, err := client.Type(context.Background(), k).Result()
			if err != nil {
				panic(err)
			}
			keyTypesCh <- keyType
		}(key)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(keyTypesCh)

	// Count the number of hashes
	hashCount := 0
	for keyType := range keyTypesCh {
		if keyType == "hash" {
			hashCount++
		}
	}
	return hashCount
}
