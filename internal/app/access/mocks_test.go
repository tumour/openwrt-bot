package access

import (
	"context"
	"fmt"

	"github.com/tumour/openwrt-bot/internal/domain"
)

// memStore — in-memory хранилище для тестов, зеркалит форму боевого адаптера:
// одно хранилище, порты наружу — sub-store'ами Users()/Roles() (в Go у одного
// типа не может быть двух методов All с разными сигнатурами).
// failUsers/failRoles инжектят инфраструктурную ошибку в операции коллекции.
type memStore struct {
	users     map[domain.UserID]domain.User
	roles     map[domain.RoleName]domain.Role
	failUsers error
	failRoles error
}

func newMemStore(users []domain.User, roles []domain.Role) *memStore {
	s := &memStore{
		users: make(map[domain.UserID]domain.User),
		roles: make(map[domain.RoleName]domain.Role),
	}
	for _, u := range users {
		s.users[u.ID] = u
	}
	for _, r := range roles {
		s.roles[r.Name] = r
	}
	return s
}

// defaultRolesStore — стор со встроенными ролями и одним активным админом (id=1).
func defaultRolesStore(users ...domain.User) *memStore {
	admin := domain.NewActiveUser(1, "admin", domain.RoleAdmin)
	return newMemStore(append([]domain.User{admin}, users...), domain.DefaultRoles())
}

func (s *memStore) Users() UserStore { return memUsers{s} }
func (s *memStore) Roles() RoleStore { return memRoles{s} }

type memUsers struct{ s *memStore }

func (m memUsers) All(context.Context) ([]domain.User, error) {
	if m.s.failUsers != nil {
		return nil, m.s.failUsers
	}
	out := make([]domain.User, 0, len(m.s.users))
	for _, u := range m.s.users {
		out = append(out, u)
	}
	return out, nil
}

func (m memUsers) Get(_ context.Context, id domain.UserID) (domain.User, error) {
	if m.s.failUsers != nil {
		return domain.User{}, m.s.failUsers
	}
	u, ok := m.s.users[id]
	if !ok {
		return domain.User{}, fmt.Errorf("%w: %d", domain.ErrUserNotFound, id)
	}
	return u, nil
}

func (m memUsers) Put(_ context.Context, u domain.User) error {
	if m.s.failUsers != nil {
		return m.s.failUsers
	}
	m.s.users[u.ID] = u
	return nil
}

func (m memUsers) Delete(_ context.Context, id domain.UserID) error {
	if m.s.failUsers != nil {
		return m.s.failUsers
	}
	if _, ok := m.s.users[id]; !ok {
		return fmt.Errorf("%w: %d", domain.ErrUserNotFound, id)
	}
	delete(m.s.users, id)
	return nil
}

type memRoles struct{ s *memStore }

func (m memRoles) All(context.Context) ([]domain.Role, error) {
	if m.s.failRoles != nil {
		return nil, m.s.failRoles
	}
	out := make([]domain.Role, 0, len(m.s.roles))
	for _, r := range m.s.roles {
		out = append(out, r)
	}
	return out, nil
}

func (m memRoles) Get(_ context.Context, name domain.RoleName) (domain.Role, error) {
	if m.s.failRoles != nil {
		return domain.Role{}, m.s.failRoles
	}
	r, ok := m.s.roles[name]
	if !ok {
		return domain.Role{}, fmt.Errorf("%w: %q", domain.ErrRoleNotFound, name)
	}
	return r, nil
}

func (m memRoles) Put(_ context.Context, r domain.Role) error {
	if m.s.failRoles != nil {
		return m.s.failRoles
	}
	m.s.roles[r.Name] = r
	return nil
}

func (m memRoles) Delete(_ context.Context, name domain.RoleName) error {
	if m.s.failRoles != nil {
		return m.s.failRoles
	}
	if _, ok := m.s.roles[name]; !ok {
		return fmt.Errorf("%w: %q", domain.ErrRoleNotFound, name)
	}
	delete(m.s.roles, name)
	return nil
}
