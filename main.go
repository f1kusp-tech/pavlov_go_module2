
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

func main() {
	user := &User{
		ID:      "1",
		Name:    "Alex",
		Balance: 1000,
	}

	for {
		fmt.Println("\n===== БАНК =====")
		fmt.Println("1. Пополнить баланс")
		fmt.Println("2. Снять деньги")
		fmt.Println("3. Посмотреть баланс")
		fmt.Println("4. Выход")
		fmt.Print("Выберите действие: ")

		var choice int
		if _, err := fmt.Scan(&choice); err != nil {
			fmt.Println("Некорректный ввод")
			return
		}

		switch choice {
		case 1:
			var amount float64
			fmt.Print("Введите сумму пополнения: ")

			if _, err := fmt.Scan(&amount); err != nil {
				fmt.Println("Некорректный ввод")
				return
			}

			if err := user.Deposit(amount); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Баланс успешно пополнен")
			}

		case 2:
			var amount float64
			fmt.Print("Введите сумму снятия: ")

			if _, err := fmt.Scan(&amount); err != nil {
				fmt.Println("Некорректный ввод")
				return
			}

			if err := user.Withdraw(amount); err != nil {
				fmt.Println("Ошибка:", err)
			} else {
				fmt.Println("Деньги успешно сняты")
			}

		case 3:
			fmt.Printf("Пользователь: %s\n", user.Name)
			fmt.Printf("ID: %s\n", user.ID)
			fmt.Printf("Текущий баланс: %.2f\n", user.Balance)

		case 4:
			fmt.Println("Выход из программы")
			return

		default:
			fmt.Println("Неизвестная команда")
		}
	}
}
