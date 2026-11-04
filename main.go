package main

import (
	"errors"
	"fmt"
)

type User struct {
	ID      string
	Name    string
	Balance float64
}

func (u *User) Deposit(amount float64) error {
	if amount <= 0 {
		return errors.New("сумма должна быть больше нуля")
	}

	u.Balance += amount
	return nil
}

func (u *User) Withdraw(amount float64) error {
	if amount <= 0 {
		return errors.New("сумма должна быть больше нуля")
	}

	if u.Balance < amount {
		return errors.New("недостаточно средств")
	}

	u.Balance -= amount
	return nil
}

func printBalance(u *User) {
	fmt.Printf("Пользователь: %s (ID: %s) | Баланс: %.2f\n", u.Name, u.ID, u.Balance)
}

func main() {
	user1 := &User{
		ID:      "1",
		Name:    "Alex",
		Balance: 1000,
	}

	user2 := &User{
		ID:      "2",
		Name:    "Maria",
		Balance: 500,
	}

	fmt.Println("===== Начальные балансы =====")
	printBalance(user1)
	printBalance(user2)

	fmt.Println("\n===== Пополнение =====")

	if err := user1.Deposit(300); err != nil {
		fmt.Println("Ошибка:", err)
	} else {
		fmt.Println("Alex пополнил баланс на 300")
		printBalance(user1)
	}

	if err := user2.Deposit(150.50); err != nil {
		fmt.Println("Ошибка:", err)
	} else {
		fmt.Println("Maria пополнила баланс на 150.50")
		printBalance(user2)
	}

	fmt.Println("\n===== Снятие =====")

	if err := user1.Withdraw(200); err != nil {
		fmt.Println("Ошибка:", err)
	} else {
		fmt.Println("Alex снял 200")
		printBalance(user1)
	}

	if err := user2.Withdraw(1000); err != nil {
		fmt.Println("Ошибка:", err)
	} else {
		fmt.Println("Maria сняла 1000")
		printBalance(user2)
	}

	fmt.Println("\n===== Итоговые балансы =====")
	printBalance(user1)
	printBalance(user2)
}
