package main

import (
	"bufio"
	"fmt"
	"net"
)

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

	//считываем поток байтов пока не встретится перевод строки
	line := make([]byte, 1024)
	n, err := buf.Read(line)
	//проверка на ошибку
	if n == 0 || err != nil {
		fmt.Println("Read error:", err)
		return
	}
	fmt.Println(string(line[:n]))

}
