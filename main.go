package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type appError string

func (e appError) Error() string { return string(e) }

const (
	ErrInvalidAmount    = appError("invalid amount")
	ErrInsufficient     = appError("insufficient funds")
	ErrSenderNotFound   = appError("sender not found")
	ErrReceiverNotFound = appError("receiver not found")
	ErrSameUser         = appError("same sender and receiver")
	ErrEmptyName        = appError("empty name")
)

type Money int64

func FormatMoney(m Money) string {
	neg := m < 0
	if neg {
		m = -m
	}
	return fmt.Sprintf("%s%d.%02d", sign(neg), int64(m)/100, int64(m)%100)
}

func sign(neg bool) string {
	if neg {
		return "-"
	}
	return ""
}

func ParseAmount(raw string) (Money, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, ErrInvalidAmount
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		return 0, ErrInvalidAmount
	}
	s = strings.ReplaceAll(s, ",", ".")

	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, ErrInvalidAmount
	}

	parts := strings.SplitN(s, ".", 2)
	rubStr := parts[0]
	if rubStr == "" {
		rubStr = "0"
	}
	rub, err := strconv.ParseInt(rubStr, 10, 64)
	if err != nil || rub < 0 {
		return 0, ErrInvalidAmount
	}

	var kop int64
	if len(parts) == 2 {
		frac := parts[1]
		if frac == "" {
			return 0, ErrInvalidAmount
		}
		if len(frac) > 2 {
			return 0, ErrInvalidAmount
		}
		frac += strings.Repeat("0", 2-len(frac))
		kop, err = strconv.ParseInt(frac, 10, 64)
		if err != nil || kop < 0 || kop > 99 {
			return 0, ErrInvalidAmount
		}
	}

	if rub > (math.MaxInt64-kop)/100 {
		return 0, ErrInvalidAmount
	}

	total := rub*100 + kop
	if neg {
		if total == 0 {
			return 0, ErrInvalidAmount
		}
		total = -total
	}
	return Money(total), nil
}

type User struct {
	id   string
	name string

	balance Money
	mu      sync.Mutex
}

func (u *User) ID() string   { return u.id }
func (u *User) Name() string { return u.name }

func (u *User) Balance() Money {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.balance
}

func (u *User) depositLocked(amount Money) {
	if amount <= 0 {
		panic("depositLocked: amount must be positive")
	}
	u.balance += amount
}

func (u *User) withdrawLocked(amount Money) error {
	if amount <= 0 {
		panic("withdrawLocked: amount must be positive")
	}
	if u.balance < amount {
		return ErrInsufficient
	}
	u.balance -= amount
	return nil
}

func (u *User) Deposit(amount Money) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.depositLocked(amount)
	return nil
}

func (u *User) Withdraw(amount Money) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.withdrawLocked(amount)
}

type Transaction struct {
	FromID string
	ToID   string
	Amount Money
}

type indexedTx struct {
	Index int
	Tx    Transaction
}

type TxResult struct {
	Index int
	Tx    Transaction
	Err   error
}

type PaymentSystem struct {
	mu     sync.RWMutex
	users  map[string]*User
	nextID int
}

func NewPaymentSystem() *PaymentSystem {
	return &PaymentSystem{
		users:  make(map[string]*User),
		nextID: 1,
	}
}

func (ps *PaymentSystem) AddUser(name string) (*User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()

	id := strconv.Itoa(ps.nextID)
	ps.nextID++
	u := &User{id: id, name: name}
	ps.users[id] = u
	return u, nil
}

func (ps *PaymentSystem) GetUser(id string) (*User, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	u, ok := ps.users[id]
	return u, ok
}

func (ps *PaymentSystem) UserCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.users)
}

func (ps *PaymentSystem) SnapshotUsers() []*User {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	users := make([]*User, 0, len(ps.users))
	for _, u := range ps.users {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].id < users[j].id
	})
	return users
}

func (ps *PaymentSystem) ProcessTransaction(t Transaction) error {
	if t.Amount <= 0 {
		return ErrInvalidAmount
	}
	if t.FromID == t.ToID {
		return ErrSameUser
	}

	ps.mu.RLock()
	from, okFrom := ps.users[t.FromID]
	to, okTo := ps.users[t.ToID]
	ps.mu.RUnlock()

	if !okFrom {
		return ErrSenderNotFound
	}
	if !okTo {
		return ErrReceiverNotFound
	}

	first, second := from, to
	if from.id > to.id {
		first, second = second, first
	}

	first.mu.Lock()
	defer first.mu.Unlock()
	second.mu.Lock()
	defer second.mu.Unlock()

	if err := from.withdrawLocked(t.Amount); err != nil {
		return err
	}
	to.depositLocked(t.Amount)
	return nil
}

type TxQueue struct {
	mu    sync.Mutex
	items []Transaction
}

func (q *TxQueue) Add(t Transaction) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, t)
}

func (q *TxQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *TxQueue) takeAll() []Transaction {
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.items
	q.items = nil
	return items
}

func (q *TxQueue) addMany(txs []Transaction) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(txs, q.items...)
}

type Processor struct {
	ps      *PaymentSystem
	queue   *TxQueue
	workers int
}

func NewProcessor(ps *PaymentSystem, q *TxQueue, workers int) *Processor {
	if workers <= 0 {
		workers = 1
	}
	return &Processor{ps: ps, queue: q, workers: workers}
}

func (p *Processor) Process() []TxResult {
	queue := p.queue.takeAll()
	if len(queue) == 0 {
		return nil
	}

	txCh := make(chan indexedTx)
	resCh := make(chan TxResult, len(queue))

	var wg sync.WaitGroup
	wg.Add(p.workers)
	for i := 0; i < p.workers; i++ {
		go p.worker(txCh, resCh, &wg)
	}

	go func() {
		defer close(txCh)
		for i, t := range queue {
			txCh <- indexedTx{Index: i, Tx: t}
		}
	}()

	go func() {
		wg.Wait()
		close(resCh)
	}()

	results := make([]TxResult, 0, len(queue))
	for r := range resCh {
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})
	return results
}

func (p *Processor) worker(
	in <-chan indexedTx,
	out chan<- TxResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for it := range in {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			err = p.ps.ProcessTransaction(it.Tx)
		}()
		out <- TxResult{Index: it.Index, Tx: it.Tx, Err: err}
	}
}

type console struct {
	scanner *bufio.Scanner
}

func newConsole() *console {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &console{scanner: sc}
}

func (c *console) readInput(prompt string) (string, bool) {
	fmt.Print(prompt)
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			fmt.Println("\nОшибка чтения ввода:", err)
		}
		return "", false
	}
	return strings.TrimSpace(c.scanner.Text()), true
}

func (c *console) readAmount() (Money, bool) {
	for {
		raw, ok := c.readInput("Введите сумму ( пусто — отмена): ")
		if !ok {
			return 0, false
		}
		if raw == "" {
			fmt.Println("Отменено.")
			return 0, false
		}
		amount, err := ParseAmount(raw)
		if err != nil || amount <= 0 {
			fmt.Println("Некорректная сумма. Попробуйте ещё раз.")
			continue
		}
		return amount, true
	}
}

func showUsers(ps *PaymentSystem) {
	fmt.Println("\nСписок пользователей:")
	for _, u := range ps.SnapshotUsers() {
		fmt.Printf("ID: %s | Имя: %s | Баланс: %s\n",
			u.ID(), u.Name(), FormatMoney(u.Balance()))
	}
}

func accountMenu(c *console, ps *PaymentSystem, proc *Processor, user *User) {
	for {
		fmt.Printf("\nУчётная запись: %s (ID: %s)\n", user.Name(), user.ID())
		fmt.Println("1. Пополнить баланс")
		fmt.Println("2. Снять деньги")
		fmt.Println("3. Перевести деньги")
		fmt.Println("4. Посмотреть баланс")
		fmt.Println("5. Обработать очередь транзакций")
		fmt.Println("0. Выйти из учётной записи")

		choice, ok := c.readInput("Выберите действие: ")
		if !ok {
			return
		}

		switch choice {
		case "1":
			amount, ok := c.readAmount()
			if !ok {
				continue
			}
			if err := user.Deposit(amount); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Баланс успешно пополнен.")
			}

		case "2":
			amount, ok := c.readAmount()
			if !ok {
				continue
			}
			if err := user.Withdraw(amount); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Деньги успешно сняты.")
			}

		case "3":
			if ps.UserCount() < 2 {
				fmt.Println("Для перевода нужен хотя бы ещё один пользователь.")
				continue
			}
			showUsers(ps)
			toID, ok := c.readInput("Введите ID получателя: ")
			if !ok {
				continue
			}
			if toID == user.ID() {
				fmt.Println("Нельзя перевести самому себе.")
				continue
			}
			if _, found := ps.GetUser(toID); !found {
				fmt.Println("Пользователь не найден.")
				continue
			}
			amount, ok := c.readAmount()
			if !ok {
				continue
			}
			proc.queue.Add(Transaction{FromID: user.ID(), ToID: toID, Amount: amount})
			fmt.Println("Транзакция добавлена в очередь.")

		case "4":
			fmt.Printf("Ваш баланс: %s\n", FormatMoney(user.Balance()))

		case "5":
			runQueue(proc)

		case "0":
			fmt.Println("Вы вышли из учётной записи.")
			return

		default:
			fmt.Println("Такого пункта меню нет.")
		}
	}
}

func runQueue(proc *Processor) {
	if proc.queue.Len() == 0 {
		fmt.Println("Очередь транзакций пуста.")
		return
	}
	results := proc.Process()
	for _, r := range results {
		status := "успешно"
		if r.Err != nil {
			status = "ошибка: " + r.Err.Error()
		}
		fmt.Printf("Транзакция %s -> %s (%s): %s\n",
			r.Tx.FromID, r.Tx.ToID, FormatMoney(r.Tx.Amount), status)
	}
}

func runSelfTests() bool {
	fmt.Println("=== SELF-TESTS ===")

	parseCases := []struct {
		in   string
		want Money
		ok   bool
	}{
		{"1.5", 150, true},
		{"1,5", 150, true},
		{"0.01", 1, true},
		{"12345678.91", 1234567891, true},
		{"0", 0, true},
		{"-5", -500, true},
		{"0.999", 0, false},
		{"0.001", 0, false},
		{"1.", 0, false},
		{"abc", 0, false},
		{"1.2.3", 0, false},
		{"", 0, false},
		{"1,2.3", 0, false},
		{"-0", 0, false},
		{"92233720368547759", 0, false},
		{"92233720368547758.07", 9223372036854775807, true},
	}
	for _, tc := range parseCases {
		got, err := ParseAmount(tc.in)
		ok := err == nil
		if ok != tc.ok {
			fmt.Printf("FAIL ParseAmount(%q): ok=%v, want ok=%v\n", tc.in, ok, tc.ok)
			return false
		}
		if ok && got != tc.want {
			fmt.Printf("FAIL ParseAmount(%q) = %d, want %d\n", tc.in, got, tc.want)
			return false
		}
	}

	if FormatMoney(12345) != "123.45" || FormatMoney(-500) != "-5.00" || FormatMoney(0) != "0.00" {
		fmt.Println("FAIL FormatMoney")
		return false
	}

	ps := NewPaymentSystem()
	alice, err := ps.AddUser("Alice")
	if err != nil {
		fmt.Println("FAIL AddUser Alice:", err)
		return false
	}
	bob, err := ps.AddUser("Bob")
	if err != nil {
		fmt.Println("FAIL AddUser Bob:", err)
		return false
	}
	if _, err := ps.AddUser("  "); err != ErrEmptyName {
		fmt.Printf("FAIL AddUser empty: got %v, want %v\n", err, ErrEmptyName)
		return false
	}

	if err := alice.Deposit(1000); err != nil {
		fmt.Println("FAIL Deposit:", err)
		return false
	}
	if err := alice.Deposit(0); err != ErrInvalidAmount {
		fmt.Printf("FAIL Deposit zero: got %v, want %v\n", err, ErrInvalidAmount)
		return false
	}
	if err := alice.Withdraw(5000); err != ErrInsufficient {
		fmt.Printf("FAIL Withdraw over: got %v, want %v\n", err, ErrInsufficient)
		return false
	}
	if err := alice.Withdraw(0); err != ErrInvalidAmount {
		fmt.Printf("FAIL Withdraw zero: got %v, want %v\n", err, ErrInvalidAmount)
		return false
	}

	if err := ps.ProcessTransaction(Transaction{alice.ID(), alice.ID(), 10}); err != ErrSameUser {
		fmt.Printf("FAIL same user: got %v, want %v\n", err, ErrSameUser)
		return false
	}
	if err := ps.ProcessTransaction(Transaction{"nope", bob.ID(), 10}); err != ErrSenderNotFound {
		fmt.Printf("FAIL sender not found: got %v, want %v\n", err, ErrSenderNotFound)
		return false
	}
	if err := ps.ProcessTransaction(Transaction{alice.ID(), "nope", 10}); err != ErrReceiverNotFound {
		fmt.Printf("FAIL receiver not found: got %v, want %v\n", err, ErrReceiverNotFound)
		return false
	}
	if err := ps.ProcessTransaction(Transaction{alice.ID(), bob.ID(), 0}); err != ErrInvalidAmount {
		fmt.Printf("FAIL zero amount: got %v, want %v\n", err, ErrInvalidAmount)
		return false
	}

	_ = alice.Deposit(100000)
	_ = bob.Deposit(100000)

	const N = 1000
	var wg sync.WaitGroup
	wg.Add(2 * N)
	for i := 0; i < N; i++ {
		go func() { defer wg.Done(); _ = ps.ProcessTransaction(Transaction{alice.ID(), bob.ID(), 1}) }()
		go func() { defer wg.Done(); _ = ps.ProcessTransaction(Transaction{bob.ID(), alice.ID(), 1}) }()
	}
	wg.Wait()
	total := alice.Balance() + bob.Balance()
	if total != 201000 {
		fmt.Printf("FAIL total=%s\n", FormatMoney(total))
		return false
	}

	queue := &TxQueue{}
	queue.Add(Transaction{alice.ID(), bob.ID(), 10})
	queue.Add(Transaction{alice.ID(), "nonexistent", 10})
	queue.Add(Transaction{alice.ID(), bob.ID(), 20})

	proc := NewProcessor(ps, queue, 3)
	results := proc.Process()
	if len(results) != 3 {
		fmt.Printf("FAIL results len=%d\n", len(results))
		return false
	}
	for i, r := range results {
		if r.Index != i {
			fmt.Printf("FAIL order at %d: index=%d\n", i, r.Index)
			return false
		}
	}
	if results[1].Err != ErrReceiverNotFound {
		fmt.Printf("FAIL expected receiver not found, got %v\n", results[1].Err)
		return false
	}

	fmt.Println("ALL SELF-TESTS PASSED")
	return true
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--selftest" {
			if !runSelfTests() {
				os.Exit(1)
			}
			return
		}
	}

	c := newConsole()
	ps := NewPaymentSystem()
	queue := &TxQueue{}
	proc := NewProcessor(ps, queue, 3)

	fmt.Println("=== ПЛАТЁЖНАЯ СИСТЕМА (v4) ===")

	for {
		fmt.Println("\n1. Создать учётную запись")
		fmt.Println("2. Войти в учётную запись")
		fmt.Println("3. Список пользователей")
		fmt.Println("4. Обработать очередь транзакций")
		fmt.Println("0. Выход")

		choice, ok := c.readInput("Выберите действие: ")
		if !ok {
			fmt.Println("\nРабота программы завершена.")
			return
		}

		switch choice {
		case "1":
			name, ok := c.readInput("Введите имя: ")
			if !ok {
				continue
			}
			user, err := ps.AddUser(name)
			if err != nil {
				fmt.Println("Ошибка:", err)
				continue
			}
			fmt.Println("Учётная запись успешно создана.")
			fmt.Println("Ваш ID:", user.ID())
			accountMenu(c, ps, proc, user)

		case "2":
			if ps.UserCount() == 0 {
				fmt.Println("Сначала создайте учётную запись.")
				continue
			}
			id, ok := c.readInput("Введите ваш ID: ")
			if !ok {
				continue
			}
			user, found := ps.GetUser(id)
			if !found {
				fmt.Println("Пользователь не найден.")
				continue
			}
			accountMenu(c, ps, proc, user)

		case "3":
			if ps.UserCount() == 0 {
				fmt.Println("Пользователей пока нет.")
			} else {
				showUsers(ps)
			}

		case "4":
			runQueue(proc)

		case "0":
			fmt.Println("Работа программы завершена.")
			return

		default:
			fmt.Println("Такого пункта меню нет.")
		}
	}
}
