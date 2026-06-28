package catalog

import (
	"strings"
)

// message types
type MessageType string

const (
	UnknownType  MessageType = "unknown"       // unknown type
	Series       MessageType = "series"        // series of related messages
	Message      MessageType = "message"       // a teaching or preached message
	Prayer       MessageType = "prayer"        // prayer before service (sunday school)
	Song         MessageType = "song"          // song
	Booklet      MessageType = "booklet"       // a stand-alone booklet
	SpecialEvent MessageType = "special-event" // wedding, funeral, child-dedication, etc
	Testimony    MessageType = "testimony"     // someone testifying about something God has done
	Training     MessageType = "training"      // general leadership training or specific ministry training
	Word         MessageType = "word"          // a prophesy, encouragment, or other utterance under the Holy Spirit
	MinistryTime MessageType = "ministry-time" // time of individual prayer, normally at the end of service
)

func NewMessageTypeFromString(s string) MessageType {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	switch s {
	case "series":
		return Series
	case "message":
		return Message
	case "prayer":
		return Prayer
	case "song":
		return Song
	case "book", "booklet":
		return Booklet
	case "special event", string(SpecialEvent):
		return SpecialEvent
	case "testimony":
		return Testimony
	case "training":
		return Training
	case "word":
		return Word
	case "ministry time":
		return MinistryTime
	}

	// log.Printf("WARNING: Encountered unknown message type '%s'", s)
	return UnknownType
}
