package security

import "golang.org/x/crypto/bcrypt"

type BcryptHasher struct {
	Cost int
}

func (h BcryptHasher) Hash(password string) (string, error) {
	cost := h.cost()
	out, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (h BcryptHasher) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (h BcryptHasher) cost() int {
	if h.Cost >= bcrypt.MinCost {
		return h.Cost
	}
	return bcrypt.DefaultCost
}
