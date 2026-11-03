package main

import (
	"bufio"
	"fmt"
	"os"
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
	ErrUserExists       = appError("user already exists")
)

type User struct {
	ID      string
	Name    string
	Balance float64
}

func (u *User) Deposit(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	u.Balance += amount
	return nil
}

func (u *User) Withdraw(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if u.Balance < amount {
		return ErrInsufficient
	}
	u.Balance -= amount
	return nil
}

type Transaction struct {
	FromID string
	ToID   string
	Amount float64
}

type PaymentSystem struct {
	Users            map[string]*User
	TransactionQueue []Transaction
	nextID           int
}

func (ps *PaymentSystem) AddUser(user *User) error {
	if ps.Users == nil {
		ps.Users = make(map[string]*User)
	}
	if _, ok := ps.Users[user.ID]; ok {
		return ErrUserExists
	}
	ps.Users[user.ID] = user
	if n, err := strconv.Atoi(user.ID); err == nil && n >= ps.nextID {
		ps.nextID = n + 1
	}
	return nil
}

func (ps *PaymentSystem) NewUser(name string) *User {
	if ps.nextID == 0 {
		ps.nextID = 1
	}
	id := strconv.Itoa(ps.nextID)
	ps.nextID++
	return &User{ID: id, Name: name, Balance: 0}
}

func (ps *PaymentSystem) AddTransaction(t Transaction) {
	ps.TransactionQueue = append(ps.TransactionQueue, t)
}

func (ps *PaymentSystem) ProcessingTransactions(t Transaction) error {
	if t.Amount <= 0 {
		return ErrInvalidAmount
	}
	if t.FromID == t.ToID {
		return ErrSameUser
	}

	fromUser, ok := ps.Users[t.FromID]
	if !ok {
		return ErrSenderNotFound
	}

	toUser, ok := ps.Users[t.ToID]
	if !ok {
		return ErrReceiverNotFound
	}

	if err := fromUser.Withdraw(t.Amount); err != nil {
		return err
	}

	toUser.Balance += t.Amount
	return nil
}

func (ps *PaymentSystem) Worker(ch <-chan Transaction, wg *sync.WaitGroup) {
	defer wg.Done()

	for t := range ch {
		if err := ps.ProcessingTransactions(t); err != nil {
			fmt.Printf("Транзакция %s -> %s (%.2f): ошибка: %v\n", t.FromID, t.ToID, t.Amount, err)
		} else {
			fmt.Printf("Транзакция %s -> %s (%.2f): успешно\n", t.FromID, t.ToID, t.Amount)
		}
	}
}

func (ps *PaymentSystem) ProcessQueue() {
	if len(ps.TransactionQueue) == 0 {
		fmt.Println("Очередь транзакций пуста.")
		return
	}

	ch := make(chan Transaction, len(ps.TransactionQueue))

	var wg sync.WaitGroup

	const workers = 3

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go ps.Worker(ch, &wg)
	}

	for _, t := range ps.TransactionQueue {
		ch <- t
	}

	close(ch)
	wg.Wait()

	ps.TransactionQueue = nil
}

var scanner = bufio.NewScanner(os.Stdin)

func readInput(prompt string) string {
	fmt.Print(prompt)
	if !scanner.Scan() {
		return "0"
	}
	return strings.TrimSpace(scanner.Text())
}

func readAmount() float64 {
	for {
		value := strings.ReplaceAll(readInput("Введите сумму: "), ",", ".")
		amount, err := strconv.ParseFloat(value, 64)

		if err == nil && amount > 0 {
			return amount
		}

		if value == "0" {
			return 0
		}

		fmt.Println("Некорректная сумма. Попробуйте ещё раз.")
	}
}

func sortedIDs(ps *PaymentSystem) []string {
	ids := make([]string, 0, len(ps.Users))
	for id := range ps.Users {
		ids = append(ids, id)
	}

	for i := 1; i < len(ids); i++ {
		key := ids[i]
		j := i - 1
		for j >= 0 && ids[j] > key {
			ids[j+1] = ids[j]
			j--
		}
		ids[j+1] = key
	}

	return ids
}

func showUsers(ps *PaymentSystem) {
	fmt.Println("\nСписок пользователей:")

	for _, id := range sortedIDs(ps) {
		user := ps.Users[id]
		fmt.Printf("ID: %s | Имя: %s | Баланс: %.2f\n",
			user.ID, user.Name, user.Balance)
	}
}

func accountMenu(ps *PaymentSystem, user *User) {
	for {
		fmt.Printf("\nУчётная запись: %s (ID: %s)\n", user.Name, user.ID)
		fmt.Println("1. Пополнить баланс")
		fmt.Println("2. Снять деньги")
		fmt.Println("3. Перевести деньги")
		fmt.Println("4. Посмотреть баланс")
		fmt.Println("5. Обработать очередь транзакций")
		fmt.Println("0. Выйти из учётной записи")

		choice := readInput("Выберите действие: ")

		switch choice {
		case "1":
			amount := readAmount()
			if amount == 0 {
				continue
			}
			if err := user.Deposit(amount); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Баланс успешно пополнен.")
			}

		case "2":
			amount := readAmount()
			if amount == 0 {
				continue
			}
			if err := user.Withdraw(amount); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Деньги успешно сняты.")
			}

		case "3":
			if len(ps.Users) < 2 {
				fmt.Println("Для перевода нужен хотя бы ещё один пользователь.")
				continue
			}

			showUsers(ps)
			toID := readInput("Введите ID получателя: ")
			if toID == user.ID {
				fmt.Println("Нельзя перевести самому себе.")
				continue
			}
			if _, ok := ps.Users[toID]; !ok {
				fmt.Println("Пользователь не найден.")
				continue
			}
			amount := readAmount()
			if amount == 0 {
				continue
			}

			transaction := Transaction{
				FromID: user.ID,
				ToID:   toID,
				Amount: amount,
			}

			ps.AddTransaction(transaction)
			fmt.Println("Транзакция добавлена в очередь.")

		case "4":
			fmt.Printf("Ваш баланс: %.2f\n", user.Balance)

		case "5":
			ps.ProcessQueue()

		case "0":
			fmt.Println("Вы вышли из учётной записи.")
			return

		default:
			fmt.Println("Такого пункта меню нет.")
		}
	}
}

func main() {
	ps := &PaymentSystem{
		Users:  make(map[string]*User),
		nextID: 1,
	}

	fmt.Println("=== ПЛАТЁЖНАЯ СИСТЕМА (v3) ===")

	for {
		fmt.Println("\n1. Создать учётную запись")
		fmt.Println("2. Войти в учётную запись")
		fmt.Println("3. Список пользователей")
		fmt.Println("4. Обработать очередь транзакций")
		fmt.Println("0. Выход")

		choice := readInput("Выберите действие: ")

		switch choice {
		case "1":
			name := readInput("Введите имя: ")
			if name == "" || name == "0" {
				fmt.Println("Имя не может быть пустым.")
				continue
			}

			user := ps.NewUser(name)
			if err := ps.AddUser(user); err != nil {
				fmt.Println("Ошибка:", err)
				continue
			}

			fmt.Println("Учётная запись успешно создана.")
			fmt.Println("Ваш ID:", user.ID)
			accountMenu(ps, user)

		case "2":
			if len(ps.Users) == 0 {
				fmt.Println("Сначала создайте учётную запись.")
				continue
			}

			id := readInput("Введите ваш ID: ")
			user, ok := ps.Users[id]
			if !ok {
				fmt.Println("Пользователь не найден.")
				continue
			}

			accountMenu(ps, user)

		case "3":
			if len(ps.Users) == 0 {
				fmt.Println("Пользователей пока нет.")
			} else {
				showUsers(ps)
			}

		case "4":
			ps.ProcessQueue()

		case "0":
			fmt.Println("Работа программы завершена.")
			return

		default:
			fmt.Println("Такого пункта меню нет.")
		}
	}
}
