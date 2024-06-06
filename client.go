package main

import (
	"bufio"
	"fmt"
	"net"
	"sync"
)

// структура сообщения
type Message struct {
	receiver string
	text     string
}

func main() {
	fmt.Println("Hello, I'm client!")

	fmt.Println("Input you login")
	var nickname string
	fmt.Scan(&nickname)

	fmt.Println("Input server address")
	var ipAddress string
	fmt.Scan(&ipAddress)
	//установка соединение с сервером по введённому <IP:port>
	conn, err := net.Dial("tcp", ipAddress)
	//проверка на ошибку при установке
	if err != nil {
		fmt.Println(err)
		return
	}
	//закрытие в конце выполнения программы
	defer conn.Close()

	conn.Write([]byte(nickname))

	//объявляем поток вывода с источником в виде подключения
	buf := bufio.NewReader(conn)

	line := make([]byte, 1024)
	//считываем поток байтов
	n, err := buf.Read(line)
	//проверка на ошибку
	if n == 0 || err != nil {
		fmt.Println("Read error:", err)
		return
	}
	fmt.Println(string(line[:n]))
	//waitGroup для синхронизации горутин
	var wg sync.WaitGroup
	wg.Add(2)
	//отправка сообщений
	go sendingMessage(conn, &wg)
	//получение сообщений

	wg.Wait()
}

// функция отправки сообщений
func sendingMessage(conn net.Conn, wg *sync.WaitGroup) {

	quit := make(chan int)
	go receivingMessage(conn, wg, quit)
	for {
		var text string
		fmt.Scan(&text)
		if text == "###" {
			wg.Done()
			close(quit)
			break
		}
		conn.Write([]byte(text))
	}
}

// функция получения сообщений
func receivingMessage(conn net.Conn, wg *sync.WaitGroup, ch chan int) {
	//объявляем поток вывода с источником в виде подключения
	buf := bufio.NewReader(conn)
	for {
		select {
		case <-ch:
			wg.Done()
			return
		default:
			line := make([]byte, 1024)
			//считываем поток байтов
			n, err := buf.Read(line)
			//проверка на ошибку
			if n == 0 || err != nil {
				fmt.Println("Read error:", err)
				wg.Done()
				return
			}
			fmt.Println(string(line[:n]))
		}
	}
}

//функции "sendingMessage" и "receivingMessage" будут выполняться
//в отдельных горутинах чтобы не блокировать ввод и вывод

//проблема:нужно как-то выходить из диалога
//2 решения: 1) топорное: ввод определённого набора символов (например: "###")
//2) через UNIX-сигналы: при нажатии сочетания клавиш ctrl+c (^C) будет произведён выход из диалога в главное меню
