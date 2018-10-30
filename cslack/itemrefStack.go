package cslack

import (
	"errors"
	"log"
	"sync"

	"github.com/nlopes/slack"
)

/* ***** SLACK MESSAGE STACK ***** */

// ItemRefStack stack of ItemRefs
type ItemRefStack struct {
	lock sync.Mutex // you don't have to do this if you don't want thread safety
	s    []slack.ItemRef
}

// NewItemRefStack returns stack of slack.ItemRef
func NewItemRefStack() *ItemRefStack {
	if *debugCSlack {
		log.Println("ItemRefStack initialized")
	}
	return &ItemRefStack{sync.Mutex{}, make([]slack.ItemRef, 0)}
}

// Push pushes onto stack
func (s *ItemRefStack) Push(v slack.ItemRef) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if *debugCSlack {
		log.Printf("ItemRefStack pushed: %s %s", v.Timestamp, v.Comment)
	}
	s.s = append(s.s, v)
}

// Pop pops from stack returns ItemRef
func (s *ItemRefStack) Pop() (slack.ItemRef, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	l := len(s.s)
	if l == 0 {
		emptyRef := slack.NewRefToMessage("", "")
		return emptyRef, errors.New("Empty Stack")
	}

	if *debugCSlack {
		log.Println("ItemRefStack popped: ", s.s[l-1])
	}

	res := s.s[l-1]
	s.s = s.s[:l-1]
	return res, nil
}

// Search return true if Channel and Timestamp match an ItemRef in the stack
func (s *ItemRefStack) Search(c string, ts string) (bool, error) {
	l := len(s.s)
	if l == 0 {
		return false, nil
	}
	for _, value := range s.s {
		if value.Channel == c && value.Timestamp == ts {
			return true, nil
		}
	}
	return false, nil
}
