package memdb

//go:generate sh -c "go run watch-gen/main.go >watch_few.go"

import (
	"context"
)

// aFew gives how many watchers this function is wired to support. You must
// always pass a full slice of this length, but unused channels can be nil.
const aFew = 32

// watchFew is used if there are only a few watchers as a performance
// optimization.
func watchFew(ctx context.Context, ch []<-chan struct{}) error {
	select {

	case <-ch[0]:
		return nil

	case <-ch[1]:
		return nil

	case <-ch[2]:
		return nil

	case <-ch[3]:
		return nil

	case <-ch[4]:
		return nil

	case <-ch[5]:
		return nil

	case <-ch[6]:
		return nil

	case <-ch[7]:
		return nil

	case <-ch[8]:
		return nil

	case <-ch[9]:
		return nil

	case <-ch[10]:
		return nil

	case <-ch[11]:
		return nil

	case <-ch[12]:
		return nil

	case <-ch[13]:
		return nil

	case <-ch[14]:
		return nil

	case <-ch[15]:
		return nil

	case <-ch[16]:
		return nil

	case <-ch[17]:
		return nil

	case <-ch[18]:
		return nil

	case <-ch[19]:
		return nil

	case <-ch[20]:
		return nil

	case <-ch[21]:
		return nil

	case <-ch[22]:
		return nil

	case <-ch[23]:
		return nil

	case <-ch[24]:
		return nil

	case <-ch[25]:
		return nil

	case <-ch[26]:
		return nil

	case <-ch[27]:
		return nil

	case <-ch[28]:
		return nil

	case <-ch[29]:
		return nil

	case <-ch[30]:
		return nil

	case <-ch[31]:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}
